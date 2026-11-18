// Package book loads a book folder into memory: its settings and its parsed
// chapters. It knows nothing about output formats.
package book

import (
	"fmt"
	"hash/fnv"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/markdown"
)

type Book struct {
	Dir      string // absolute path of the book folder
	Config   *config.Config
	Chapters []*Chapter // every chapter in reading order
	Nav      []*Node    // the same chapters arranged in parts and sections
	Created  string     // YYYY-MM-DD: "created:" in octavo.yml, else the first git commit; may be ""
	Updated  string     // YYYY-MM-DD: the newest chapter date
	Added    []string   // chapters added by themselves, not listed in octavo.yml (see autoadd.go)
	NotAdded []NotAdded // chapter files that are not in the book and could not be added
}

// Node is one line of the chapter list: a chapter, or a group with children.
type Node struct {
	Title    string
	Chapter  *Chapter // nil for a group
	Children []*Node
}

type Chapter struct {
	Path    string // source path relative to the book folder, with forward slashes
	Title   string
	Number  int      // 0 means unnumbered
	Hash    uint64   // fingerprint of the expanded source
	Updated string   // YYYY-MM-DD: the newest date of the chapter and the files it includes
	Files   []string // the chapter's file and every file it includes, relative to the book
	Doc     *markdown.Document
}

// Cache keeps parsed chapters between loads, so a reload only parses the
// files that changed on disk. A chapter counts as unchanged when its file and
// every file it includes have the same modification time and size, and the
// settings it uses (variables, book details) are the same.
type Cache struct {
	entries map[string]cached
	git     *gitLog
}

type cached struct {
	deps     []dep
	settings string
	hash     uint64
	files    []string
	doc      *markdown.Document
}

type dep struct {
	file     string
	modified time.Time
	size     int64
}

func NewCache() *Cache { return &Cache{entries: map[string]cached{}} }

func (c *Cache) fresh(e cached, settings string) bool {
	if e.settings != settings {
		return false
	}
	for _, d := range e.deps {
		info, err := os.Stat(d.file)
		if err != nil || !info.ModTime().Equal(d.modified) || info.Size() != d.size {
			return false
		}
	}
	return true
}

// parse returns the parsed chapter at rel, from the cache when possible.
func (c *Cache) parse(x *expander, rel string) (*markdown.Document, uint64, []string, error) {
	key := filepath.Join(x.bookDir, filepath.FromSlash(rel))
	settings := x.settingsKey()
	if c != nil {
		if e, ok := c.entries[key]; ok && c.fresh(e, settings) {
			return e.doc, e.hash, e.files, nil
		}
	}
	src, err := x.expand(rel)
	if err != nil {
		return nil, 0, nil, err
	}
	doc, err := markdown.Parse(src)
	if err != nil {
		return nil, 0, nil, err
	}
	h := fnv.New64a()
	h.Write(src)
	files := append([]string(nil), x.files...)
	if c != nil {
		var deps []dep
		for _, f := range files {
			full := filepath.Join(x.bookDir, filepath.FromSlash(f))
			if info, err := os.Stat(full); err == nil {
				deps = append(deps, dep{full, info.ModTime(), info.Size()})
			}
		}
		c.entries[key] = cached{deps, settings, h.Sum64(), files, doc}
	}
	return doc, h.Sum64(), files, nil
}

// Load reads octavo.yml and every chapter it lists.
func Load(dir string) (*Book, error) { return LoadCached(dir, nil) }

// LoadCached is Load, reusing parsed chapters from cache. cache may be nil.
func LoadCached(dir string, cache *Cache) (*Book, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(abs)
	if err != nil {
		return nil, err
	}
	b := &Book{Dir: abs, Config: cfg}
	x := newExpander(abs, cfg)
	// Adding chapters walks the list twice; a cache saves parsing twice.
	if cache == nil {
		cache = NewCache()
	}
	var prevGit *gitLog
	if cache != nil {
		prevGit = cache.git
	}
	gitInfo := readGit(abs, prevGit)
	if cache != nil && gitInfo != nil {
		cache.git = gitInfo.gitLog
	}
	number := 0
	var walk func(entries []config.Entry) ([]*Node, error)
	walk = func(entries []config.Entry) ([]*Node, error) {
		var nodes []*Node
		for _, e := range entries {
			if e.Group != "" {
				children, err := walk(e.Contents)
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, &Node{Title: e.Group, Children: children})
				continue
			}
			rel := filepath.ToSlash(e.File)
			doc, hash, files, err := cache.parse(x, rel)
			if err != nil {
				return nil, fmt.Errorf("chapter %s: %w", rel, err)
			}
			ch := &Chapter{Path: rel, Doc: doc, Hash: hash, Files: files, Title: doc.Meta.Title}
			for _, f := range files {
				if d := gitInfo.day(abs, f); d > ch.Updated {
					ch.Updated = d
				}
			}
			if ch.Updated > b.Updated {
				b.Updated = ch.Updated
			}
			if ch.Title == "" {
				ch.Title = doc.Title
			}
			if ch.Title == "" {
				ch.Title = e.Text
			}
			if ch.Title == "" {
				ch.Title = strings.TrimSuffix(path.Base(rel), ".md")
			}
			// A leading index.md is the preface and stays unnumbered.
			preface := len(b.Chapters) == 0 && path.Base(rel) == "index.md"
			if !preface && !doc.Meta.Unnumbered {
				number++
				ch.Number = number
			}
			b.Chapters = append(b.Chapters, ch)
			label := e.Text
			if label == "" {
				label = ch.Title
			}
			nodes = append(nodes, &Node{Title: label, Chapter: ch})
		}
		return nodes, nil
	}
	if b.Nav, err = walk(cfg.Book.Chapters); err != nil {
		return nil, err
	}
	if cfg.Book.AutoAdd == nil || *cfg.Book.AutoAdd {
		listed := map[string]bool{}
		for _, ch := range b.Chapters {
			for _, f := range ch.Files {
				listed[f] = true
			}
		}
		b.Added, b.NotAdded = autoAdd(abs, b.OutputDir(), &cfg.Book.Chapters, listed)
		if len(b.Added) > 0 {
			b.Chapters, b.Updated, number = nil, "", 0
			if b.Nav, err = walk(cfg.Book.Chapters); err != nil {
				return nil, err
			}
		}
	}
	b.Created = cfg.Book.Created
	if b.Created == "" && gitInfo != nil {
		b.Created = gitInfo.first
	}
	return b, nil
}

// OutputDir is the absolute folder that "octavo render" writes to.
func (b *Book) OutputDir() string {
	if filepath.IsAbs(b.Config.OutputDir) {
		return b.Config.OutputDir
	}
	return filepath.Join(b.Dir, b.Config.OutputDir)
}
