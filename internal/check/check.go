// Package check finds problems in a book's sources without building it:
// broken links, missing images, images without descriptions, chapter files
// left out of the chapter list, and images nothing uses.
package check

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kanthi/octavo/internal/book"
	"github.com/kanthi/octavo/internal/config"
)

// Problem is one thing check found.
type Problem struct {
	Error   bool   // an error breaks the book; a warning is worth a look
	File    string // relative to the book folder
	Message string
}

func (p Problem) String() string {
	level := "warning"
	if p.Error {
		level = "error"
	}
	return fmt.Sprintf("%-8s %s: %s", level, p.File, p.Message)
}

var imageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".avif": true}

// Book checks the book in dir. A book that does not load at all (a missing
// chapter, a bad include, an unknown variable) is one error.
func Book(dir string) (*book.Book, []Problem) {
	b, err := book.Load(dir)
	if err != nil {
		return nil, []Problem{{Error: true, File: config.FileName, Message: err.Error()}}
	}
	var problems []Problem
	add := func(isErr bool, file, format string, args ...any) {
		problems = append(problems, Problem{Error: isErr, File: file, Message: fmt.Sprintf(format, args...)})
	}

	chapters := map[string]*book.Chapter{}
	used := map[string]bool{} // every file a chapter reads or points at
	for _, ch := range b.Chapters {
		chapters[ch.Path] = ch
		for _, f := range ch.Files {
			used[f] = true
		}
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(b.Dir, filepath.FromSlash(rel)))
		return err == nil
	}

	for _, ch := range b.Chapters {
		for _, dest := range ch.Doc.Links {
			u, err := url.Parse(dest)
			if err != nil {
				add(true, ch.Path, "link %q cannot be read as an address", dest)
				continue
			}
			if u.Scheme != "" || u.Host != "" || strings.HasPrefix(u.Path, "/") {
				continue // web addresses and absolute paths are not checked
			}
			if u.Path == "" { // "#part": a place in this chapter
				if u.Fragment != "" && !ch.Doc.IDs[u.Fragment] {
					add(true, ch.Path, "link %q: no heading or figure with that id in this chapter%s", dest, suggest(u.Fragment, ch.Doc.IDs))
				}
				continue
			}
			target := path.Join(path.Dir(ch.Path), u.Path)
			if !filepath.IsLocal(filepath.FromSlash(target)) {
				add(false, ch.Path, "link %q points outside the book folder", dest)
				continue
			}
			used[target] = true
			if strings.HasSuffix(target, ".md") {
				other, ok := chapters[target]
				switch {
				case !ok && exists(target):
					add(true, ch.Path, "link %q: that file is not in the chapter list", dest)
				case !ok:
					add(true, ch.Path, "link %q: no such chapter", dest)
				case u.Fragment != "" && !other.Doc.IDs[u.Fragment]:
					add(true, ch.Path, "link %q: %s has no heading or figure with id %q%s", dest, target, u.Fragment, suggest(u.Fragment, other.Doc.IDs))
				}
				continue
			}
			if !exists(target) {
				add(true, ch.Path, "link %q: no such file", dest)
			}
		}
		for _, img := range ch.Doc.Pictures {
			u, err := url.Parse(img.Dest)
			local := err == nil && u.Scheme == "" && u.Host == "" && u.Path != "" && !strings.HasPrefix(u.Path, "/")
			if local {
				target := path.Join(path.Dir(ch.Path), u.Path)
				used[target] = true
				if !exists(target) {
					add(true, ch.Path, "image %q: no such file", img.Dest)
				}
			}
			if strings.TrimSpace(img.Alt) == "" {
				add(false, ch.Path, "image %q has no description; write one between the brackets: ![what it shows](%s)", img.Dest, img.Dest)
			}
		}
	}

	// Files nothing uses: Markdown files left out of the chapter list, and
	// images no chapter shows. A file named anywhere in the settings, a
	// stylesheet, a template or a chapter counts as used.
	mentions := corpus(b)
	output := filepath.Clean(b.Config.OutputDir)
	var unlisted, unused []string
	_ = filepath.WalkDir(b.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(b.Dir, p)
		name := d.Name()
		// Names starting with "." or "_" are set aside on purpose: notes,
		// drafts, files to include.
		if rel != "." && (strings.HasPrefix(name, ".") || name == "node_modules" || rel == output ||
			strings.HasPrefix(name, "_") && (d.IsDir() || strings.EqualFold(path.Ext(name), ".md"))) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if _, err := os.Stat(filepath.Join(p, config.FileName)); err == nil && rel != "." {
				return filepath.SkipDir // another book inside this one
			}
			return nil
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(path.Ext(rel))
		switch {
		case ext == ".md" && !used[rel] && !strings.EqualFold(name, "README.md"):
			unlisted = append(unlisted, rel)
		case imageExt[ext] && !used[rel] && !mentioned(rel, mentions, used):
			unused = append(unused, rel)
		}
		return nil
	})
	sort.Strings(unlisted)
	sort.Strings(unused)
	why := map[string]string{}
	for _, n := range b.NotAdded {
		why[n.Path] = n.Reason
	}
	for _, f := range unlisted {
		if r, ok := why[f]; ok {
			add(false, f, "%s", book.NotAddedMessage(r))
			continue
		}
		add(false, f, "not in the chapter list, and no chapter includes it")
	}
	for _, f := range unused {
		add(false, f, "no chapter, stylesheet or setting uses this image")
	}
	return b, problems
}

// corpus is the text of the book's settings, stylesheets, templates and
// chapter sources, where a file's name counts as a use of it.
func corpus(b *book.Book) string {
	var s strings.Builder
	read := func(rel string) {
		if data, err := os.ReadFile(filepath.Join(b.Dir, filepath.FromSlash(rel))); err == nil {
			s.Write(data)
			s.WriteByte('\n')
		}
	}
	read(config.FileName)
	for _, ch := range b.Chapters {
		for _, f := range ch.Files {
			read(f)
		}
	}
	_ = filepath.WalkDir(b.Dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if ext := filepath.Ext(p); ext == ".css" || ext == ".html" {
				rel, _ := filepath.Rel(b.Dir, p)
				if !strings.HasPrefix(filepath.ToSlash(rel), filepath.ToSlash(filepath.Clean(b.Config.OutputDir))+"/") {
					read(rel)
				}
			}
		}
		return nil
	})
	return s.String()
}

// mentioned reports whether an image is named somewhere, or is the dark-page
// version of an image that is used.
func mentioned(rel, mentions string, used map[string]bool) bool {
	ext := path.Ext(rel)
	if base := strings.TrimSuffix(rel, ext); strings.HasSuffix(base, "-dark") {
		if light := strings.TrimSuffix(base, "-dark") + ext; used[light] || strings.Contains(mentions, path.Base(light)) {
			return true
		}
	}
	return strings.Contains(mentions, rel) || strings.Contains(mentions, path.Base(rel))
}

// suggest names an id that differs from the wanted one only in punctuation,
// such as "go-127" for "go-1.27".
func suggest(want string, ids map[string]bool) string {
	squash := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, strings.ToLower(s))
	}
	for id := range ids {
		if squash(id) == squash(want) {
			return fmt.Sprintf(" (did you mean #%s?)", id)
		}
	}
	return ""
}
