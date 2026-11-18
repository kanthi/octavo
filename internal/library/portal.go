package library

import (
	"bytes"
	_ "embed"
	"hash/fnv"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/format/fonts"
	"github.com/kanthi/octavo/internal/fsutil"
)

//go:embed portal.html
var portalHTML string

var portalTemplate = template.Must(template.New("portal").Parse(portalHTML))

type card struct {
	Name        string // folder name
	Title       string
	Description string
	Author      string
	Chapters    int
	Updated     string     // YYYY-MM-DD, the book's newest chapter date
	URL         string     // what the card opens: the website, else the first download
	Read        string     // the website copy, when there is one
	Downloads   []download // the PDF and EPUB files, when they were built
	Draft       bool       // shown with a "Draft" badge; only built with --drafts
	Colour      colour     // the card's colour, different for every book (see colours)
	Search      string     // lower-case text the search box matches against
}

type download struct{ Label, URL string }

type portal struct {
	Settings
	Books       []card
	Stylesheets []string
	LiveReload  bool
	FontFaces   template.CSS // @font-face rules for the title font
}

// bookFacts returns a book's chapter count and newest chapter date: from
// this run's build, or, for a book not built in this run, by loading it.
// ok is false when the book does not load.
func (l *Library) bookFacts(name string) (f bookFacts, ok bool) {
	if f, ok := l.facts[name]; ok {
		return f, true
	}
	b, err := book.Load(filepath.Join(l.BooksDir(), name))
	if err != nil {
		return bookFacts{}, false
	}
	l.setFacts(name, b)
	return l.facts[name], true
}

// titleFont sets the book titles on the cards.
var titleFont, _ = fonts.Find("literata")

// writeFonts copies the title font into the public folder, skipping files
// that are already there unchanged.
func writeFonts(dir string) error {
	for _, name := range fonts.Names() {
		if !strings.HasPrefix(name, "literata-") {
			continue
		}
		data := fonts.Read(name)
		file := filepath.Join(dir, name)
		if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := fsutil.WriteFile(file, data); err != nil {
			return err
		}
	}
	return nil
}

// colour is a card's colour: a hue in OKLCH (degrees) and a tone, 0 for
// pale and 1 for deep. The portal's stylesheet turns it into the card's
// tints.
type colour struct{ Hue, Tone int }

// lab is the colour's cover tint in OKLab, where distance is how different
// two colours look. Lightness counts less than hue: two hues tell books apart
// more readily than two shades of one hue.
func (c colour) lab() (l, a, b float64) {
	l, ch := 0.86, 0.10
	if c.Tone == 1 {
		l, ch = 0.72, 0.135
	}
	rad := float64(c.Hue) * math.Pi / 180
	return l * 0.6, ch * math.Cos(rad), ch * math.Sin(rad)
}

func (c colour) distance(o colour) float64 {
	l1, a1, b1 := c.lab()
	l2, a2, b2 := o.lab()
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// colours gives every book a colour that looks as different as possible from
// all the others. Books are taken in order; each gets the colour farthest
// from every colour already taken, so adding a book never changes the
// colours of the books before it. While there is room, no two books share a
// hue: each is at least hueGap degrees from the others, and the tone only
// adds contrast. Placed one at a time, that holds for at least 14 books;
// after that a hue comes back, in the other tone where it can. The first book's colour comes from its
// name, and so do ties.
func colours(names []string) map[string]colour {
	var candidates []colour
	for tone := 0; tone <= 1; tone++ {
		for hue := 0; hue < 360; hue += 5 {
			candidates = append(candidates, colour{hue, tone})
		}
	}
	const hueGap = 20
	hueFree := func(c colour, taken []colour) bool {
		for _, t := range taken {
			d := math.Abs(float64(c.Hue - t.Hue))
			if math.Min(d, 360-d) < hueGap {
				return false
			}
		}
		return true
	}
	var taken []colour
	out := map[string]colour{}
	for _, name := range names {
		pool := candidates[:0:0]
		for _, c := range candidates {
			if hueFree(c, taken) {
				pool = append(pool, c)
			}
		}
		if len(pool) == 0 {
			pool = candidates
		}
		h := fnv.New32a()
		h.Write([]byte(name))
		home := int(h.Sum32() % 360)
		best, bestScore := candidates[0], math.Inf(-1)
		for _, c := range pool {
			nearest := 1.0
			for _, t := range taken {
				nearest = math.Min(nearest, c.distance(t))
			}
			d := math.Abs(float64(c.Hue - home))
			d = math.Min(d, 360-d)
			// Ties: the pale tone, then the hue nearest the name's own.
			score := nearest - float64(c.Tone)*1e-4 - d*1e-7
			if score > bestScore {
				best, bestScore = c, score
			}
		}
		taken = append(taken, best)
		out[name] = best
	}
	return out
}

// writePortal writes public/index.html. It lists every book of the library
// that has a copy in the public folder, whether or not this run built it.
// Drafts are listed only when drafts is true. It reports whether the page
// changed.
func (l *Library) writePortal(public string, liveReload, drafts bool) (bool, error) {
	p := portal{Settings: l.Config.Library, LiveReload: liveReload, FontFaces: template.CSS(fonts.CSS("assets/fonts/", titleFont))}
	if err := writeFonts(filepath.Join(public, "assets", "fonts")); err != nil {
		return false, err
	}
	colour := colours(l.Books)
	for _, name := range l.Books {
		if l.Draft(name) && !drafts {
			continue
		}
		var read string
		var downloads []download
		if _, err := os.Stat(filepath.Join(public, "html", name, "index.html")); err == nil {
			read = "html/" + name + "/index.html"
		}
		for _, kind := range Downloads {
			if _, err := os.Stat(filepath.Join(public, kind, name+"."+kind)); err == nil {
				downloads = append(downloads, download{strings.ToUpper(kind), kind + "/" + name + "." + kind})
			}
		}
		if read == "" && len(downloads) == 0 {
			continue
		}
		cfg, err := config.Load(filepath.Join(l.BooksDir(), name))
		if err != nil {
			continue // its old copy stays reachable by URL, but a broken book gets no card
		}
		facts, ok := l.bookFacts(name)
		if !ok {
			facts.chapters = len(cfg.Book.Files())
		}
		c := card{
			Name: name, Title: cfg.Book.Title, Description: cfg.Book.Description, Author: cfg.Book.Author,
			Chapters: facts.chapters, URL: read, Read: read, Downloads: downloads,
			Draft: l.Draft(name), Colour: colour[name], Updated: facts.updated,
		}
		if c.URL == "" {
			c.URL = downloads[0].URL
		}
		c.Search = strings.ToLower(strings.Join([]string{name, c.Title, c.Description, c.Author}, " "))
		p.Books = append(p.Books, c)
	}
	for _, css := range l.Config.Library.CSS {
		rel := filepath.ToSlash(css)
		if !filepath.IsLocal(css) {
			continue
		}
		if err := fsutil.CopyFile(filepath.Join(l.Dir, css), filepath.Join(public, "assets", filepath.FromSlash(rel))); err != nil {
			return false, err
		}
		p.Stylesheets = append(p.Stylesheets, "assets/"+rel)
	}
	var out bytes.Buffer
	if err := portalTemplate.Execute(&out, p); err != nil {
		return false, err
	}
	file := filepath.Join(public, "index.html")
	if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, out.Bytes()) {
		return false, nil
	}
	return true, fsutil.WriteFile(file, out.Bytes())
}
