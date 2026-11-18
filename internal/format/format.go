// Package format defines what an output format is and keeps the list of
// available ones. To add a format (EPUB, PDF, ...): write a package with a
// Factory, then register it in internal/cli/modules.go.
package format

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
)

// Options are the per-run settings every format receives.
type Options struct {
	LiveReload bool      // preview mode: pages reload when the book is rebuilt
	Log        io.Writer // warnings; never nil inside Render

	// HomeURL, when set, adds a link from every page to a parent site, such
	// as a library portal. It is relative to the folder the book is served from.
	HomeURL   string
	HomeLabel string

	// SiteURL, when set, is the address the book will be served from, ending
	// in "/". It overrides "site-url:" in octavo.yml; a library sets it.
	SiteURL string
	// InSite is true when the book is part of a larger site (a library) that
	// writes its own sitemap.xml and 404 page.
	InSite bool

	// Formats names the formats to render. Empty means the ones enabled in
	// octavo.yml. A named format that octavo.yml does not mention is rendered
	// with its default settings.
	Formats []string

	// DownloadURL, when set, says where a book file (PDF, EPUB) will be
	// served, relative to the folder the book is served from. A library sets
	// it because it moves those files out of the book's folder. The default
	// is the file's own name, beside the website.
	DownloadURL func(kind, file string) string
	// Downloads are the book files built in this run. RenderAll fills it in,
	// so the HTML book can link to them.
	Downloads []Download
	// Versions, when the book is built in several versions, lists them all
	// for the website's version menu.
	Versions []Version
}

// Version is one entry of the version menu.
type Version struct {
	Label   string
	URL     string // that version's folder, relative to this version's folder ("../1st/")
	Current bool   // the version being built
	Latest  bool   // the first version listed, which the site's address leads to
}

// Download is a link to the book as one file.
type Download struct {
	Label string // "PDF", "EPUB"
	URL   string
}

// Checker is implemented by formats that need something installed on this
// machine. Check says what is missing.
type Checker interface {
	Check() error
}

// Filer is implemented by formats that write the book as one file. File is
// that file's name in the output folder.
type Filer interface {
	File(b *book.Book) string
}

// Format writes a book into out. It should give every file a fingerprint of
// its inputs (see Output), so a rebuild only redoes the files that changed.
type Format interface {
	Render(b *book.Book, out *Output, opts Options) error
}

// Factory builds a Format from its settings under "format:" in octavo.yml.
type Factory func(settings yaml.Node) (Format, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }

// Names lists the registered formats in alphabetical order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Select turns a list such as "html,pdf" or "all" into format names, in
// alphabetical order. An empty list returns nil, which means "the formats
// enabled in octavo.yml".
func Select(list string) ([]string, error) {
	chosen := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		switch {
		case name == "":
		case name == "all":
			return Names(), nil
		case registry[name] == nil:
			return nil, fmt.Errorf("unknown format %q (choose from: all, %s)", name, strings.Join(Names(), ", "))
		default:
			chosen[name] = true
		}
	}
	if len(chosen) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(chosen))
	for n := range chosen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// Partial reports whether names leaves out some available format. A partial
// build keeps the files already in the output folder, so that rendering only
// the PDF does not delete the HTML book next to it.
func Partial(names []string) bool {
	return len(names) > 0 && len(names) < len(registry)
}

// RenderAll renders every format enabled in octavo.yml into out, or the ones
// named in opts.Formats. A book with no "format:" section gets every format. With a new
// Output this is a clean, full build. With an Output kept from an earlier
// build, only the files whose inputs changed are written again.
func RenderAll(b *book.Book, out *Output, opts Options) error {
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	// A misspelt format in octavo.yml is an error even when --to leaves it out.
	for name := range b.Config.Format {
		if _, ok := registry[name]; !ok {
			return fmt.Errorf("octavo.yml: format %q is not available (available: %s)", name, strings.Join(Names(), ", "))
		}
	}
	var formats []named
	names := opts.Formats
	chosen := len(names) > 0
	if !chosen {
		for n := range b.Config.Format {
			names = append(names, n)
		}
		sort.Strings(names)
		// A book that says nothing about formats gets all of them.
		if len(names) == 0 {
			names = Names()
		}
	}
	for _, name := range names {
		node := b.Config.Format[name] // the zero node, when octavo.yml has no settings for it
		if !chosen && !config.Enabled(node) {
			continue
		}
		factory, ok := registry[name]
		if !ok {
			return fmt.Errorf("format %q is not available (available: %s)", name, strings.Join(Names(), ", "))
		}
		f, err := factory(node)
		if err != nil {
			return fmt.Errorf("format %s: %w", name, err)
		}
		// A format that cannot run here (PDF without a typesetting tool) is
		// an error when it was asked for by name, and a warning otherwise.
		if checker, ok := f.(Checker); ok {
			if err := checker.Check(); err != nil {
				if chosen {
					return fmt.Errorf("%s: %w", name, err)
				}
				fmt.Fprintf(opts.Log, "warning: %s skipped: %v\n", name, err)
				continue
			}
		}
		formats = append(formats, named{name, f})
	}
	if len(formats) == 0 {
		return fmt.Errorf("no format is enabled in %s", config.FileName)
	}

	// Chapter files the book could not take in by themselves.
	for _, n := range b.NotAdded {
		fmt.Fprintf(opts.Log, "warning: %s: %s\n", n.Path, book.NotAddedMessage(n.Reason))
	}

	// A first build empties the output folder, so it must not hold the book itself.
	dir, err := filepath.Abs(out.Dir)
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(dir, b.Dir); err == nil && filepath.IsLocal(rel) {
		return fmt.Errorf("output-dir %s contains the book folder; choose a folder inside the book, such as _book", dir)
	}
	opts.Downloads = nil
	for _, f := range formats {
		if filer, ok := f.Format.(Filer); ok {
			url := filer.File(b)
			if opts.DownloadURL != nil {
				url = opts.DownloadURL(f.name, url)
			}
			opts.Downloads = append(opts.Downloads, Download{Label: strings.ToUpper(f.name), URL: url})
		}
	}
	out.Dir = dir
	if err := out.begin(Partial(opts.Formats)); err != nil {
		return err
	}
	for _, f := range formats {
		if err := f.Render(b, out, opts); err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return out.end()
}

type named struct {
	name string
	Format
}
