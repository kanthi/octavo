// Package cli is the command line: it parses arguments and calls the other
// packages. It holds no book logic of its own.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/guide"
	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/check"
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/html"
	"github.com/kanthi/octavo/internal/library"
	"github.com/kanthi/octavo/internal/preview"
	"github.com/kanthi/octavo/internal/publish"
	"github.com/kanthi/octavo/internal/scaffold"
	"github.com/kanthi/octavo/internal/versions"
)

// Version is the release name printed by "octavo version".
var Version = "0.1.1"

const usage = `Octavo writes books in Markdown and publishes them.

Usage:
  octavo new <dir>           Create a new book in <dir>
  octavo new --library <dir> Create a library: a folder for several books
             [--books A,B]   ...with these books in it
  octavo add <Name>...       Add new books to the library in this folder
  octavo add --from <book>   Copy an existing book into the library; the
             [<Name>]        original is left as it is
  octavo render              Build the book into its output folder (_book)
  octavo preview             Serve the book locally; on every save, rebuild only
                             what changed and reload the browser ("dev" also works)
  octavo publish [target]    Build the book, then send it to a publish target
  octavo publish --setup github
                             Write a GitHub Actions workflow that publishes the
                             book on GitHub Pages on every push
  octavo check               Look for broken links, missing images and other
                             problems, without building anything
  octavo guide               Open the Octavo Guide, the full user guide, in
                             your browser (--to pdf or epub to save a copy,
                             --copy <dir> for its source as a sample book)
  octavo version             Print the version

"new" is only for starting out; it refuses to run inside a library. Later
books are added with "add", which never touches a book that already exists.

In a library folder (one with octavo-library.yml) the same commands work on
every book: each builds into its own _book, is copied into the public folder,
and a portal page lists them all. "octavo render Linux Go" builds only those.

Drafts: a book with "draft: true" under "book:" in its octavo.yml is left out
of render, preview and publish, and off the portal. --drafts includes it.

Formats: a book is built as a website (html), an e-book (epub) and a PDF
(pdf). "render" and "publish" build the formats listed under "format:" in
octavo.yml, or all three when it lists none. --to picks them for one run
instead: --to pdf, --to html,epub, --to all. PDF is set by Typst when it
is installed, else printed by Chrome, Chromium, Edge or Brave; with neither
it is skipped with a warning.

New chapters: a Markdown file beside the book's chapters that octavo.yml
does not list joins the book by its number (06-sensors.md). Files without a
number are reported. "auto-add: false" under "book:" turns this off.

Options:
  --dir <path>     The book or library folder (default: the current folder)
  --drafts         render, preview, publish: include books marked as drafts
  --to <formats>   render, publish: all, or a comma-separated list of
                   html, epub, pdf (default: $OCTAVO_TO, then octavo.yml)
  --port <number>  preview: port to serve on (default 4000)
  --open           preview: open the book in your browser
  --no-render      publish: send the output folder as it is, without rebuilding
  --setup github   publish: write .github/workflows/octavo.yml instead of publishing
`

// Run executes one command and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	book.OctavoVersion = Version
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	commands := map[string]func([]string, io.Writer, io.Writer) error{
		"new": cmdNew, "add": cmdAdd, "render": cmdRender, "preview": cmdPreview, "dev": cmdPreview, "publish": cmdPublish, "guide": cmdGuide, "check": cmdCheck,
		"version": func(_ []string, out, _ io.Writer) error {
			fmt.Fprintln(out, "octavo", Version)
			return nil
		},
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "octavo: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	if err := cmd(args[1:], stdout, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "octavo:", err)
		return 1
	}
	return 0
}

// parse reads flags that may come before or after positional arguments.
func parse(fs *flag.FlagSet, args []string, stderr io.Writer) ([]string, error) {
	fs.SetOutput(stderr)
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func cmdNew(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	lib := fs.Bool("library", false, "create a library for several books")
	books := fs.String("books", "", "with --library: comma-separated names of books to create in it")
	pos, err := parse(fs, args, stderr)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("usage: octavo new [--library [--books A,B]] <dir>")
	}
	dir := pos[0]
	if above := libraryAbove(dir); above != "" {
		if wd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(wd, above); err == nil {
				above = rel
			}
		}
		return fmt.Errorf("%s is inside the library %s; add books to it with:\n  octavo add --dir %s <Name>", dir, above, above)
	}
	if !*lib {
		if *books != "" {
			return fmt.Errorf("--books only works with --library")
		}
		if err := scaffold.New(dir); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created a book in %s\n\nNext:\n  cd %s\n  octavo preview\n", dir, dir)
		return nil
	}

	names, err := checkNewNames(splitList(*books), nil)
	if err != nil {
		return err
	}
	if err := scaffold.NewLibrary(dir); err != nil {
		return err
	}
	for _, name := range names {
		if err := scaffold.NewInLibrary(filepath.Join(dir, "books", name)); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "Created a library in %s\n", dir)
	for _, name := range names {
		fmt.Fprintf(stdout, "  added book %s\n", filepath.Join("books", name))
	}
	next := "  octavo add My-Book\n"
	if len(names) > 0 {
		next = ""
	}
	fmt.Fprintf(stdout, "\nNext:\n  cd %s\n%s  octavo preview\n", dir, next)
	return nil
}

func cmdAdd(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	dir := fs.String("dir", ".", "the library folder")
	from := fs.String("from", "", "an existing book folder to copy into the library")
	pos, err := parse(fs, args, stderr)
	if err != nil {
		return err
	}
	if !library.Is(*dir) {
		if _, err := os.Stat(filepath.Join(*dir, "octavo.yml")); err == nil {
			return fmt.Errorf("%s is a single book, not a library. To make a library from it, leaving this book as it is:\n"+
				"  octavo new --library <new-folder>\n  octavo add --dir <new-folder> --from %s", *dir, *dir)
		}
		return fmt.Errorf("no library in %s (no %s); create one with \"octavo new --library <dir>\", or pass --dir", *dir, library.FileName)
	}
	l, err := library.Open(*dir)
	if err != nil {
		return err
	}

	if *from != "" {
		switch len(pos) {
		case 0:
			abs, err := filepath.Abs(*from)
			if err != nil {
				return err
			}
			pos = []string{filepath.Base(abs)}
		case 1:
		default:
			return fmt.Errorf("usage: octavo add --from <book> [<Name>] (one book at a time)")
		}
		names, err := checkNewNames(pos, l)
		if err != nil {
			return err
		}
		target := filepath.Join(l.BooksDir(), names[0])
		n, err := scaffold.CopyBook(*from, target)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Copied %d files from %s into %s. The original was not changed.\n", n, *from, l.Rel(target))
		if cfg, err := book.Load(target); err == nil {
			if len(cfg.Config.Publish) > 0 {
				fmt.Fprintf(stdout, "Note: its \"publish:\" settings are not used inside a library; \"octavo publish\" uses %s.\n", library.FileName)
			}
			if cfg.Config.Book.Draft {
				fmt.Fprintln(stdout, "Note: it is marked as a draft, so it is built only with --drafts.")
			}
		}
		return nil
	}

	if len(pos) == 0 {
		return fmt.Errorf("usage: octavo add <Name>... (or: octavo add --from <book> [<Name>])")
	}
	names, err := checkNewNames(pos, l)
	if err != nil {
		return err
	}
	for _, name := range names {
		target := filepath.Join(l.BooksDir(), name)
		if err := scaffold.NewInLibrary(target); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Added book %s\n", l.Rel(target))
	}
	return nil
}

// checkNewNames checks every name before anything is created, so a bad name
// does not leave half the books made. With a library, names already taken
// there (in any letter case, by a book or by any other file) are refused.
func checkNewNames(names []string, l *library.Library) ([]string, error) {
	seen := map[string]bool{}
	for _, name := range names {
		if err := scaffold.ValidBookName(name); err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if seen[key] {
			return nil, fmt.Errorf("book %q is named twice", name)
		}
		seen[key] = true
		if l == nil {
			continue
		}
		if have, ok := l.Find(name); ok {
			return nil, fmt.Errorf("the library already has a book %q; Octavo never overwrites a book", have)
		}
		if _, err := os.Lstat(filepath.Join(l.BooksDir(), name)); err == nil {
			return nil, fmt.Errorf("%s already exists; Octavo never overwrites it", l.Rel(filepath.Join(l.BooksDir(), name)))
		}
	}
	return names, nil
}

// libraryAbove returns the library folder that holds dir (dir itself
// included), or "" when there is none.
func libraryAbove(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if library.Is(abs) {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cmdRender(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	dir := fs.String("dir", ".", "the book folder")
	to := toFlag(fs)
	drafts := draftsFlag(fs)
	jobs := jobsFlag(fs)
	pos, err := parse(fs, args, stderr)
	if err != nil {
		return err
	}
	formats, err := format.Select(*to)
	if err != nil {
		return err
	}
	if library.Is(*dir) {
		_, err := renderLibrary(*dir, pos, formats, *drafts, *jobs, stdout)
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("naming books (%s) only works in a library folder", strings.Join(pos, ", "))
	}
	_, err = render(*dir, formats, *drafts, stdout, stderr)
	return err
}

// toFlag adds --to, with --format as another name for it.
func toFlag(fs *flag.FlagSet) *string {
	def := os.Getenv("OCTAVO_TO")
	to := fs.String("to", def, "formats to build: all, or a comma-separated list of "+strings.Join(format.Names(), ", "))
	// The same default again: StringVar writes its default into the shared
	// variable, so "" here would erase $OCTAVO_TO.
	fs.StringVar(to, "format", def, "another name for --to")
	return to
}

func draftsFlag(fs *flag.FlagSet) *bool {
	return fs.Bool("drafts", false, "include books marked \"draft: true\"")
}

func jobsFlag(fs *flag.FlagSet) *int {
	return fs.Int("jobs", library.DefaultJobs(), "in a library, how many books to build at the same time")
}

// renderLibrary builds the named books of a library, or all of them.
func renderLibrary(dir string, only, formats []string, drafts bool, jobs int, stdout io.Writer) (*library.Library, error) {
	l, err := library.Load(dir)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(stdout, "Library %q\n", l.Config.Library.Title)
	res, err := l.Build(library.BuildOptions{Only: only, Formats: formats, Drafts: drafts, Jobs: jobs, Log: stdout})
	if err != nil {
		return nil, err
	}
	skipped := ""
	if n := len(res.Drafts); n > 0 {
		skipped = fmt.Sprintf(", skipped %d %s (%s)", n, plural(n, "draft", "drafts"), strings.Join(res.Drafts, ", "))
	}
	fmt.Fprintf(stdout, "Built %d of %d books%s. Portal: %s\n", len(res.Built), len(l.Books), skipped, filepath.Join(res.Public, "index.html"))
	return l, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// loadBook loads a single book, refusing a draft unless drafts is set.
func loadBook(dir string, drafts bool) (*book.Book, error) {
	b, err := book.Load(dir)
	if err != nil {
		return nil, err
	}
	if b.Config.Book.Draft && !drafts {
		return nil, fmt.Errorf("this book is a draft (draft: true in its octavo.yml); add --drafts to build it")
	}
	return b, nil
}

func render(dir string, formats []string, drafts bool, stdout, stderr io.Writer) (*book.Book, error) {
	b, err := loadBook(dir, drafts)
	if err != nil {
		return nil, err
	}
	changed, err := versions.Build(b, b.OutputDir(), format.Options{Formats: formats, Log: stderr})
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(stdout, "Built %d chapters into %s\n", len(b.Chapters), b.OutputDir())
	if n := len(b.Added); n > 0 {
		fmt.Fprintf(stdout, "  %s not in %s, added by number: %s\n", plural(n, "1 chapter", fmt.Sprint(n)+" chapters"), "octavo.yml", strings.Join(b.Added, ", "))
	}
	if n := len(b.Config.Versions); n > 0 {
		fmt.Fprintf(stdout, "  in %d versions, the latest in %s/\n", n, versions.Latest(b))
	}
	for _, file := range changed {
		if ext := filepath.Ext(file); ext == ".pdf" || ext == ".epub" {
			fmt.Fprintf(stdout, "  %-5s %s\n", strings.ToUpper(ext[1:]), filepath.Join(b.OutputDir(), file))
		}
	}
	return b, nil
}

// The preview is a website, so it builds only the HTML book, whatever
// octavo.yml enables. Printing a PDF on every save would be far too slow.
var previewFormats = []string{"html"}

func cmdPreview(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	dir := fs.String("dir", ".", "the book folder")
	port := fs.Int("port", 4000, "port to serve on")
	open := fs.Bool("open", false, "open the book in your browser")
	drafts := draftsFlag(fs)
	if _, err := parse(fs, args, stderr); err != nil {
		return err
	}
	// Both build functions keep their state between runs, so each rebuild
	// only redoes the files whose inputs changed.
	var build func(out string) ([]string, error)
	if library.Is(*dir) {
		if _, err := library.Load(*dir); err != nil { // fail early on a broken library
			return err
		}
		state := library.NewState()
		build = func(out string) ([]string, error) {
			l, err := library.Load(*dir)
			if err != nil {
				return nil, err
			}
			res, err := l.Build(library.BuildOptions{PublicDir: out, Direct: true, LiveReload: true, Log: stdout, State: state, Formats: previewFormats, Drafts: *drafts})
			return res.Changed, err
		}
	} else {
		if _, err := loadBook(*dir, *drafts); err != nil { // fail early when there is no book here
			return err
		}
		cache := book.NewCache()
		var output *format.Output
		build = func(out string) ([]string, error) {
			if output == nil {
				output = format.NewOutput(out)
			}
			b, err := book.LoadCached(*dir, cache)
			if err != nil {
				return nil, err
			}
			if b.Config.Book.Draft && !*drafts {
				return nil, fmt.Errorf("this book is now a draft (draft: true in its octavo.yml); restart the preview with --drafts")
			}
			err = format.RenderAll(b, output, format.Options{LiveReload: true, Log: stdout, Formats: previewFormats})
			return output.Changed(), err
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return preview.Run(ctx, *dir, fmt.Sprintf("127.0.0.1:%d", *port), stdout, build, func(url string) {
		fmt.Fprintf(stdout, "Preview at %s (Ctrl+C to stop)\n", url)
		if *open {
			openBrowser(url)
		}
	})
}

func cmdPublish(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	dir := fs.String("dir", ".", "the book folder")
	noRender := fs.Bool("no-render", false, "send the output folder as it is")
	setup := fs.String("setup", "", "write the files that publish from a hosting service: github")
	to := toFlag(fs)
	drafts := draftsFlag(fs)
	jobs := jobsFlag(fs)
	pos, err := parse(fs, args, stderr)
	if err != nil {
		return err
	}
	formats, err := format.Select(*to)
	if err != nil {
		return err
	}
	if *setup != "" {
		return setupPublishing(*setup, *dir, *drafts, stdout)
	}
	// A library publishes its public folder; a book publishes its output folder.
	var configured map[string]yaml.Node
	var siteDir, sourceDir string
	isLibrary := library.Is(*dir)
	if isLibrary {
		l, err := library.Load(*dir)
		if err != nil {
			return err
		}
		configured, siteDir, sourceDir = l.Config.Publish, l.PublicDir(), l.Dir
	} else {
		b, err := loadBook(*dir, *drafts)
		if err != nil {
			return err
		}
		configured, siteDir, sourceDir = b.Config.Publish, b.OutputDir(), b.Dir
	}

	targets := make([]string, 0, len(configured))
	for name := range configured {
		targets = append(targets, name)
	}
	sort.Strings(targets)
	var name string
	switch {
	case len(pos) > 1:
		return fmt.Errorf("usage: octavo publish [target]")
	case len(pos) == 1:
		name = pos[0]
	case len(targets) == 1:
		name = targets[0]
	case len(targets) == 0:
		return fmt.Errorf("no publish target is configured; add one under \"publish:\" (types: %s)", strings.Join(publish.Kinds(), ", "))
	default:
		return fmt.Errorf("choose a target: octavo publish <%s>", strings.Join(targets, "|"))
	}
	settings, ok := configured[name]
	if !ok {
		return fmt.Errorf("no publish target %q (found: %s)", name, strings.Join(targets, ", "))
	}
	target, err := publish.New(name, settings)
	if err != nil {
		return err
	}

	switch {
	case *noRender:
		if _, err := os.Stat(siteDir); err != nil {
			return fmt.Errorf("nothing to publish: %s does not exist; run \"octavo render\" first", siteDir)
		}
	case isLibrary:
		if _, err := renderLibrary(*dir, nil, formats, *drafts, *jobs, stdout); err != nil {
			return err
		}
	default:
		if _, err := render(*dir, formats, *drafts, stdout, stderr); err != nil {
			return err
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Fprintf(stdout, "Publishing to %s...\n", name)
	if err := target.Publish(ctx, publish.Site{Dir: siteDir, BookDir: sourceDir, Log: stdout}); err != nil {
		return fmt.Errorf("publish %s: %w", name, err)
	}
	fmt.Fprintln(stdout, "Published.")
	return nil
}

// setupPublishing writes what a hosting service needs to build and publish
// the book on its own, on every push.
func setupPublishing(service, dir string, drafts bool, stdout io.Writer) error {
	if service != "github" {
		return fmt.Errorf("--setup: unknown service %q (available: github)", service)
	}
	var output string
	if library.Is(dir) {
		l, err := library.Load(dir)
		if err != nil {
			return err
		}
		output = l.PublicDir()
	} else {
		b, err := loadBook(dir, drafts)
		if err != nil {
			return err
		}
		output = b.OutputDir()
	}
	g, err := scaffold.GitHubWorkflow(dir, output)
	if err != nil {
		return err
	}
	settings, site := "Settings > Pages", "your GitHub Pages address"
	if g.Owner != "" {
		settings = fmt.Sprintf("https://github.com/%s/%s/settings/pages", g.Owner, g.Name)
		site = fmt.Sprintf("https://%s.github.io/%s/", strings.ToLower(g.Owner), g.Name)
	}
	fmt.Fprintf(stdout, `Wrote %s

Next:
  1. On GitHub, open %s and set
     "Build and deployment > Source" to "GitHub Actions".
  2. Commit the workflow and push it:
       git add %s
       git commit -m "Publish with Octavo"
       git push

Then every push to %s builds the book and publishes it at %s
(progress: the Actions tab of the repository).

Commit %s too, whenever it changes: Octavo keeps it beside
octavo.yml so that the published site forwards old addresses of moved chapters.
`, g.Workflow, settings, scaffold.WorkflowPath, g.Branch, site, html.PagesFile)
	return nil
}

func cmdGuide(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("guide", flag.ContinueOnError)
	to := fs.String("to", "", "save the guide as a file instead of opening it: pdf, epub, or pdf,epub")
	out := fs.String("out", ".", "with --to: the folder to save the files in")
	copyTo := fs.String("copy", "", "write the guide's source into this new folder, as a book to edit and render")
	port := fs.Int("port", 0, "port to serve on (default: any free port)")
	noOpen := fs.Bool("no-open", false, "do not open the browser")
	pos, err := parse(fs, args, stderr)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("usage: octavo guide [--to pdf,epub [--out <dir>]] [--copy <dir>]")
	}

	if *copyTo != "" {
		if _, err := os.Lstat(*copyTo); err == nil {
			return fmt.Errorf("%s already exists; Octavo never overwrites it", *copyTo)
		}
		if err := guide.WriteTo(*copyTo); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote the Octavo Guide's source to %s. It is an ordinary book:\n  cd %s\n  octavo preview\n", *copyTo, *copyTo)
		return nil
	}

	src, err := os.MkdirTemp("", "octavo-guide-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(src)
	if err := guide.WriteTo(src); err != nil {
		return err
	}

	if *to != "" {
		formats, err := format.Select(*to)
		if err != nil {
			return err
		}
		for _, f := range formats {
			if f != "pdf" && f != "epub" {
				return fmt.Errorf("--to: the guide can be saved as pdf or epub; without --to it opens as a website")
			}
		}
		b, err := book.Load(src)
		if err != nil {
			return err
		}
		output := format.NewOutput(b.OutputDir())
		if err := format.RenderAll(b, output, format.Options{Formats: formats, Log: stderr}); err != nil {
			return err
		}
		for _, file := range output.Changed() {
			ext := filepath.Ext(file)
			if ext != ".pdf" && ext != ".epub" {
				continue
			}
			target := filepath.Join(*out, "Octavo-Guide-"+Version+ext)
			if _, err := os.Lstat(target); err == nil {
				return fmt.Errorf("%s already exists; delete it first or choose another folder with --out", target)
			}
			if err := os.MkdirAll(*out, 0o755); err != nil {
				return err
			}
			data, err := os.ReadFile(filepath.Join(b.OutputDir(), file))
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Saved %s\n", target)
		}
		return nil
	}

	cache := book.NewCache()
	var output *format.Output
	build := func(dir string) ([]string, error) {
		if output == nil {
			output = format.NewOutput(dir)
		}
		b, err := book.LoadCached(src, cache)
		if err != nil {
			return nil, err
		}
		err = format.RenderAll(b, output, format.Options{Log: stderr, Formats: previewFormats})
		return output.Changed(), err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return preview.Run(ctx, src, fmt.Sprintf("127.0.0.1:%d", *port), io.Discard, build, func(url string) {
		fmt.Fprintf(stdout, "The Octavo Guide (%s) is at %s (Ctrl+C to stop)\n", Version, url)
		if !*noOpen {
			openBrowser(url)
		}
	})
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", url).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func cmdCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	dir := fs.String("dir", ".", "the book or library folder")
	quiet := fs.Bool("quiet", false, "report errors only, not warnings")
	pos, err := parse(fs, args, stderr)
	if err != nil {
		return err
	}
	type target struct{ label, dir string }
	var targets []target
	if library.Is(*dir) {
		l, err := library.Load(*dir)
		if err != nil {
			return err
		}
		names := l.Books
		if len(pos) > 0 {
			names = nil
			for _, want := range pos {
				found, ok := l.Find(want)
				if !ok {
					return fmt.Errorf("no book %q in this library (books: %s)", want, strings.Join(l.Books, ", "))
				}
				names = append(names, found)
			}
		}
		for _, name := range names {
			label := name
			if l.Draft(name) {
				label += " (draft)"
			}
			targets = append(targets, target{label, filepath.Join(l.BooksDir(), name)})
		}
	} else {
		if len(pos) > 0 {
			return fmt.Errorf("naming books (%s) only works in a library folder", strings.Join(pos, ", "))
		}
		targets = append(targets, target{"", *dir})
	}

	errs, warns := 0, 0
	for _, t := range targets {
		b, problems := check.Book(t.dir)
		title := t.label
		if b != nil {
			title = fmt.Sprintf("%q, %d chapters", b.Config.Book.Title, len(b.Chapters))
			if t.label != "" {
				title = t.label + ": " + title
			}
		}
		fmt.Fprintf(stdout, "Checking %s\n", title)
		shown := 0
		for _, p := range problems {
			if p.Error {
				errs++
			} else {
				warns++
				if *quiet {
					continue
				}
			}
			fmt.Fprintf(stdout, "  %s\n", p)
			shown++
		}
		if shown == 0 {
			fmt.Fprintln(stdout, "  no problems found")
		}
	}
	fmt.Fprintf(stdout, "%d %s, %d %s.\n", errs, plural(errs, "error", "errors"), warns, plural(warns, "warning", "warnings"))
	if errs > 0 {
		return fmt.Errorf("check found %d %s", errs, plural(errs, "error", "errors"))
	}
	return nil
}
