// Package library handles a folder of several books. Each book builds into
// its own output folder, then is copied into one public folder that also
// holds a portal page listing every book.
//
//	my-library/
//	├── octavo-library.yml
//	├── books/
//	│   ├── Linux/      a normal book: octavo.yml, chapters, _book/
//	│   └── Go/
//	└── public/         the site to host
//	    ├── index.html  the portal
//	    └── html/Linux/, html/Go/
package library

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/html"
	"github.com/kanthi/octavo/internal/fsutil"
	"github.com/kanthi/octavo/internal/versions"
)

// FileName marks a folder as a library.
const FileName = "octavo-library.yml"

type Config struct {
	Library Settings             `yaml:"library"`
	Publish map[string]yaml.Node `yaml:"publish"`
}

type Settings struct {
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	BooksDir    string            `yaml:"books-dir"`  // folder holding one subfolder per book
	OutputDir   string            `yaml:"output-dir"` // the public folder
	Theme       string            `yaml:"theme"`      // light, dark or auto
	SiteURL     string            `yaml:"site-url"`   // the site's address, for the sitemap and link previews
	CSS         config.StringList `yaml:"css"`        // your stylesheets for the portal page
}

type Library struct {
	Dir    string // absolute path of the library folder
	Config *Config
	Books  []string // book folder names, in alphabetical order
	drafts map[string]bool
	mu     sync.Mutex           // guards facts, while books build in parallel
	facts  map[string]bookFacts // book -> what its portal card shows, for books loaded in this run
}

// bookFacts is what a portal card says about a book that comes from its
// chapters rather than its octavo.yml.
type bookFacts struct {
	chapters int    // every chapter, including those added by number
	updated  string // YYYY-MM-DD, the newest chapter date
}

func (l *Library) setFacts(name string, b *book.Book) {
	l.mu.Lock()
	l.facts[name] = bookFacts{chapters: len(b.Chapters), updated: b.Updated}
	l.mu.Unlock()
}

// Is reports whether dir is a library folder.
func Is(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
}

// Load reads octavo-library.yml and finds the books. A library with no books
// is an error; use Open where an empty library is fine.
func Load(dir string) (*Library, error) {
	l, err := Open(dir)
	if err != nil {
		return nil, err
	}
	if len(l.Books) == 0 {
		return nil, fmt.Errorf("no books in %s yet; add one with \"octavo add <Name>\"", l.rel(l.BooksDir()))
	}
	return l, nil
}

// Open reads octavo-library.yml and finds the books, if there are any.
func Open(dir string) (*Library, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	file := filepath.Join(abs, FileName)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	c := Config{Library: Settings{Title: "Books", BooksDir: "books", OutputDir: "public", Theme: "auto"}}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if u := os.Getenv("OCTAVO_SITE_URL"); u != "" {
		c.Library.SiteURL = u
	}
	switch c.Library.Theme {
	case "light", "dark", "auto":
	default:
		return nil, fmt.Errorf("%s: theme must be light, dark or auto, not %q", file, c.Library.Theme)
	}
	l := &Library{Dir: abs, Config: &c, drafts: map[string]bool{}, facts: map[string]bookFacts{}}
	entries, err := os.ReadDir(l.BooksDir())
	if err != nil {
		return nil, fmt.Errorf("books folder: %w", err)
	}
	for _, e := range entries {
		bookDir := filepath.Join(l.BooksDir(), e.Name())
		if _, err := os.Stat(filepath.Join(bookDir, config.FileName)); err == nil && e.IsDir() {
			l.Books = append(l.Books, e.Name())
			// A book whose settings do not load is not a draft; building it
			// reports the error.
			if cfg, err := config.Load(bookDir); err == nil && cfg.Book.Draft {
				l.drafts[e.Name()] = true
			}
		}
	}
	sort.Strings(l.Books)
	return l, nil
}

// Draft reports whether a book is marked "draft: true" in its octavo.yml.
func (l *Library) Draft(name string) bool { return l.drafts[name] }

// Find returns the folder name of a book, matched in any letter case.
func (l *Library) Find(name string) (string, bool) {
	for _, have := range l.Books {
		if strings.EqualFold(have, name) {
			return have, true
		}
	}
	return "", false
}

func (l *Library) BooksDir() string  { return l.abs(l.Config.Library.BooksDir) }
func (l *Library) PublicDir() string { return l.abs(l.Config.Library.OutputDir) }

func (l *Library) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(l.Dir, p)
}

// BuildOptions control one build of the library.
type BuildOptions struct {
	Only       []string  // book names to build; empty means every book
	Drafts     bool      // also build books marked "draft: true"; without it they are skipped
	Formats    []string  // formats to build; empty means the ones each book enables
	PublicDir  string    // overrides the library's public folder (used by preview)
	Direct     bool      // build straight into the public folder, leaving each book's own output folder alone
	LiveReload bool      // preview mode
	Log        io.Writer // progress and warnings
	Jobs       int       // books built at the same time; 0 means a sensible default

	// State, used with Direct, makes builds incremental: keep one State
	// between builds and each build only redoes the files that changed.
	State *State
}

// State is what a library remembers between incremental builds.
type State struct {
	mu      sync.Mutex // guards books, while books build in parallel
	started bool
	books   map[string]*bookState
}

type bookState struct {
	out   *format.Output
	cache *book.Cache
}

func NewState() *State { return &State{books: map[string]*bookState{}} }

// Result says what a build did.
type Result struct {
	Built   []string
	Failed  []string
	Drafts  []string // draft books left out of this build
	Public  string
	Changed []string // files written in the public folder, relative to it (Direct builds only)
}

// Build renders books and refreshes the portal. A full build empties the
// public folder first. Building selected books replaces only their copies and
// keeps the other books that are already in the public folder. A book that
// fails does not stop the others; Build then returns an error at the end.
func (l *Library) Build(opts BuildOptions) (Result, error) {
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	public := l.PublicDir()
	if opts.PublicDir != "" {
		public = opts.PublicDir
	}
	res := Result{Public: public}

	var names []string
	if len(opts.Only) > 0 {
		for _, want := range opts.Only {
			found, ok := l.Find(want)
			if !ok {
				return res, fmt.Errorf("no book %q in this library (books: %s)", want, strings.Join(l.Books, ", "))
			}
			if l.Draft(found) && !opts.Drafts {
				return res, fmt.Errorf("%s is a draft (draft: true in its %s); add --drafts to build it", found, config.FileName)
			}
			if !slices.Contains(names, found) {
				names = append(names, found)
			}
		}
	} else {
		for _, name := range l.Books {
			if l.Draft(name) && !opts.Drafts {
				res.Drafts = append(res.Drafts, name)
				fmt.Fprintf(opts.Log, "  %-16s skipped: draft (add --drafts to build it)\n", name)
				continue
			}
			names = append(names, name)
		}
		if len(names) == 0 {
			return res, fmt.Errorf("nothing to build: every book is a draft (%s); add --drafts to build them", strings.Join(res.Drafts, ", "))
		}
	}

	switch {
	case len(opts.Only) > 0:
		// Selected books only: the other books' copies stay.
	case opts.State != nil && opts.State.started:
		// An incremental build: drop books that left the library or became drafts.
		for name := range opts.State.books {
			if !slices.Contains(names, name) {
				delete(opts.State.books, name)
				if err := os.RemoveAll(filepath.Join(public, "html", name)); err != nil {
					return res, err
				}
			}
		}
	case !format.Partial(opts.Formats):
		// The public folder is emptied, so it must not hold the library itself.
		if rel, err := filepath.Rel(public, l.Dir); err == nil && filepath.IsLocal(rel) {
			return res, fmt.Errorf("output-dir %s contains the library folder; choose a folder inside it, such as public", public)
		}
		if err := fsutil.EmptyDir(public); err != nil {
			return res, err
		}
	}

	// Without --drafts, no copy of a draft may stay in the public folder, where
	// "octavo publish --no-render" would send it.
	if !opts.Drafts {
		for _, name := range l.Books {
			if l.Draft(name) {
				if err := removeCopies(public, name); err != nil {
					return res, err
				}
			}
		}
	}

	if opts.State != nil {
		opts.State.started = true
	}

	// Books are independent, so several build at once: a PDF waits on the
	// browser, and the browser uses its own processes. Each book writes its
	// messages to its own buffer; they are shown in the books' order as soon
	// as every book before them has finished.
	type outcome struct {
		log     bytes.Buffer
		changed []string
		err     error
	}
	results := make([]*outcome, len(names))
	done := make(chan int)
	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = DefaultJobs()
	}
	slots := make(chan struct{}, jobs)
	for i, name := range names {
		results[i] = &outcome{}
		go func() {
			slots <- struct{}{}
			defer func() { <-slots; done <- i }()
			o := results[i]
			bookOpts := opts
			bookOpts.Log = &o.log
			o.changed, o.err = l.buildBook(name, public, bookOpts)
		}()
	}
	finished, next := make([]bool, len(names)), 0
	for range names {
		finished[<-done] = true
		for ; next < len(names) && finished[next]; next++ {
			name, o := names[next], results[next]
			opts.Log.Write(o.log.Bytes())
			for _, rel := range o.changed {
				res.Changed = append(res.Changed, "html/"+name+"/"+rel)
			}
			if o.err != nil {
				fmt.Fprintf(opts.Log, "  %-16s FAILED: %v\n", name, o.err)
				res.Failed = append(res.Failed, name)
				continue
			}
			res.Built = append(res.Built, name)
		}
	}
	portalChanged, err := l.writePortal(public, opts.LiveReload, opts.Drafts)
	if err != nil {
		return res, err
	}
	if portalChanged {
		res.Changed = append(res.Changed, "index.html")
	}
	extras, err := l.writeSiteFiles(public)
	if err != nil {
		return res, err
	}
	res.Changed = append(res.Changed, extras...)
	if len(res.Failed) > 0 {
		return res, fmt.Errorf("%d of %d books failed: %s", len(res.Failed), len(names), strings.Join(res.Failed, ", "))
	}
	return res, nil
}

// DefaultJobs is how many books build at once unless told otherwise: one
// per two processor cores, at least 1 and at most 4. Each PDF runs a browser,
// which is heavy on memory, so more rarely helps.
func DefaultJobs() int {
	return min(max(runtime.NumCPU()/2, 1), 4)
}

// buildBook renders one book and returns the files it wrote, for Direct builds.
func (l *Library) buildBook(name, public string, opts BuildOptions) ([]string, error) {
	site := filepath.Join(public, "html", name)
	siteURL := ""
	if u := l.Config.Library.SiteURL; u != "" {
		siteURL = strings.TrimSuffix(u, "/") + "/html/" + name + "/"
	}
	renderOpts := format.Options{
		LiveReload: opts.LiveReload, Log: opts.Log, Formats: opts.Formats,
		InSite: true, SiteURL: siteURL,
		// Books sit two folders below the portal: html/<Name>/.
		HomeURL: "../../index.html", HomeLabel: l.Config.Library.Title,
		// The PDF and EPUB are moved to public/pdf and public/epub (see below).
		DownloadURL: func(kind, _ string) string { return "../../" + kind + "/" + name + "." + kind },
	}
	bookDir := filepath.Join(l.BooksDir(), name)

	if opts.Direct {
		st := &bookState{out: format.NewOutput(site), cache: book.NewCache()}
		if opts.State != nil {
			opts.State.mu.Lock()
			if old, ok := opts.State.books[name]; ok {
				st = old
			}
			opts.State.books[name] = st
			opts.State.mu.Unlock()
		}
		b, err := book.LoadCached(bookDir, st.cache)
		if err != nil {
			return nil, err
		}
		l.setFacts(name, b)
		err = format.RenderAll(b, st.out, renderOpts)
		return st.out.Changed(), err
	}

	b, err := book.Load(bookDir)
	if err != nil {
		return nil, err
	}
	l.setFacts(name, b)
	out := b.OutputDir()
	if _, err := versions.Build(b, out, renderOpts); err != nil {
		return nil, err
	}
	if err := fsutil.EmptyDir(site); err != nil {
		return nil, err
	}
	if _, err := fsutil.CopyTree(out, site); err != nil {
		return nil, err
	}
	// The book files move out of the website copy into public/pdf and
	// public/epub, named after the book's folder.
	extras := ""
	for _, kind := range Downloads {
		target := filepath.Join(public, kind, name+"."+kind)
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		found, _ := filepath.Glob(filepath.Join(site, versions.Latest(b), "*."+kind))
		if len(found) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(found[0], target); err != nil {
			return nil, err
		}
		extras += " + " + l.rel(target)
	}
	fmt.Fprintf(opts.Log, "  %-16s %d chapters  %s -> %s%s\n", name, len(b.Chapters), l.rel(out), l.rel(site), extras)
	if n := len(b.Added); n > 0 {
		fmt.Fprintf(opts.Log, "  %-16s + %d not in octavo.yml, added by number\n", "", n)
	}
	return nil, nil
}

// writeSiteFiles writes the site-wide 404 page, and the sitemap of every
// book's pages when the site's address is known. It returns the files it
// changed, relative to the public folder.
func (l *Library) writeSiteFiles(public string) ([]string, error) {
	files := map[string][]byte{"404.html": html.NotFound(l.Config.Library.Title, l.Config.Library.SiteURL)}
	if u := l.Config.Library.SiteURL; u != "" {
		u = strings.TrimSuffix(u, "/") + "/"
		pages := []string{"index.html"}
		err := filepath.WalkDir(filepath.Join(public, "html"), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(p) == ".html" && d.Name() != "404.html" && !html.IsRedirect(p) {
				rel, _ := filepath.Rel(public, p)
				pages = append(pages, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		files["sitemap.xml"] = html.Sitemap(u, pages)
	}
	var changed []string
	for name, data := range files {
		file := filepath.Join(public, name)
		if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := fsutil.WriteFile(file, data); err != nil {
			return nil, err
		}
		changed = append(changed, name)
	}
	sort.Strings(changed)
	return changed, nil
}

// removeCopies deletes a book's website, PDF and EPUB from the public folder.
func removeCopies(public, name string) error {
	if err := os.RemoveAll(filepath.Join(public, "html", name)); err != nil {
		return err
	}
	for _, kind := range Downloads {
		if err := os.Remove(filepath.Join(public, kind, name+"."+kind)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Downloads are the single-file formats. A library keeps them beside the
// website copies, as public/<kind>/<Book>.<kind>.
var Downloads = []string{"pdf", "epub"}

// Rel shortens a path for messages: relative to the library when inside it.
func (l *Library) Rel(p string) string { return l.rel(p) }

// rel shortens a path for messages.
func (l *Library) rel(p string) string {
	if r, err := filepath.Rel(l.Dir, p); err == nil && filepath.IsLocal(r) {
		return r
	}
	return p
}
