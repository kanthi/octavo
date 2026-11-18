package book

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/kanthi/octavo/internal/config"
)

// New chapters are added to the book by themselves. A Markdown file next to
// the book's chapters that octavo.yml does not list joins the book when its
// name starts with a number ("05-swap.md", "01a-why.md"): it goes after the
// chapter of the same folder with the next lower number. A new numbered
// folder beside the book's chapter folders becomes a new part or section in
// the same way, with its numbered files as chapters. octavo.yml itself is
// never changed.
//
// Files without a number are not added, because there is no telling where
// they belong; the build reports them (see NotAdded). Files and folders whose
// names start with "_" or ".", README.md files, and files that a chapter
// includes are left alone, as is everything when "auto-add: false".

// NotAdded is a Markdown file that looks like a chapter but is not in the book.
type NotAdded struct {
	Path   string // relative to the book, with forward slashes
	Reason string
}

// NotAddedMessage explains a file that was left out, and what to do.
func NotAddedMessage(reason string) string {
	return "not added to the book: " + reason + ". Give it a number, such as 05-name.md, or list it in " + config.FileName
}

var numbered = regexp.MustCompile(`^(\d+)`)

// order is a file or folder name's place among its siblings: index.md
// first, then by number, then by name. ok is false for names without a number.
func order(name string) (n int, ok bool) {
	if strings.EqualFold(name, "index.md") {
		return -1, true
	}
	m := numbered.FindString(name)
	if m == "" {
		return 0, false
	}
	n, err := strconv.Atoi(m)
	return n, err == nil
}

func before(a, b string) bool {
	na, _ := order(a)
	nb, _ := order(b)
	if na != nb {
		return na < nb
	}
	return a < b
}

func skipName(name string) bool {
	return strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || strings.EqualFold(name, "README.md")
}

// autoAdd adds unlisted numbered chapters to entries, in place. listed holds
// every chapter file and every file a chapter includes. It returns the files
// added, and the ones that could not be.
func autoAdd(bookDir, outputDir string, entries *[]config.Entry, listed map[string]bool) (added []string, notAdded []NotAdded) {
	// The folders that hold listed chapters.
	folders := map[string]bool{}
	for f := range listed {
		folders[path.Dir(f)] = true
	}
	skipDir := func(rel string) bool {
		if rel == "." {
			return false
		}
		abs := filepath.Join(bookDir, filepath.FromSlash(rel))
		if abs == outputDir {
			return true
		}
		_, err := os.Stat(filepath.Join(abs, config.FileName)) // another book
		return err == nil
	}

	// New files in the chapter folders.
	var dirs []string
	for d := range folders {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	parents := map[string]bool{}
	for _, dir := range dirs {
		if dir != "." {
			parents[path.Dir(dir)] = true
		}
		for _, name := range mdFiles(bookDir, dir) {
			rel := path.Join(dir, name)
			if listed[rel] {
				continue
			}
			if _, ok := order(name); !ok {
				notAdded = append(notAdded, NotAdded{rel, "its name has no number, so Octavo cannot tell where it goes"})
				continue
			}
			if insertChapter(entries, dir, rel) {
				listed[rel] = true
				added = append(added, rel)
			}
		}
	}

	// New numbered folders beside the chapter folders.
	var ps []string
	for p := range parents {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	for _, parent := range ps {
		for _, name := range subDirs(bookDir, parent) {
			dir := path.Join(parent, name)
			if folders[dir] || skipDir(dir) || hasListed(listed, dir) {
				continue
			}
			files := chaptersIn(bookDir, dir, &notAdded)
			if len(files) == 0 {
				continue
			}
			if _, ok := order(name); !ok {
				for _, f := range files {
					notAdded = append(notAdded, NotAdded{f, "its folder's name has no number, so Octavo cannot tell where it goes"})
				}
				continue
			}
			group := newGroup(bookDir, dir, files)
			if !insertGroup(entries, parent, dir, group) {
				for _, f := range files {
					notAdded = append(notAdded, NotAdded{f, "it is in a new folder, and Octavo cannot tell where that folder goes; list it in " + config.FileName})
				}
				continue
			}
			for _, f := range files {
				listed[f] = true
				added = append(added, f)
			}
		}
	}
	return added, notAdded
}

func hasListed(listed map[string]bool, dir string) bool {
	for f := range listed {
		if strings.HasPrefix(f, dir+"/") {
			return true
		}
	}
	return false
}

// mdFiles lists the chapter-like Markdown files in dir, in book order.
func mdFiles(bookDir, dir string) []string {
	entries, _ := os.ReadDir(filepath.Join(bookDir, filepath.FromSlash(dir)))
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(path.Ext(e.Name()), ".md") && !skipName(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return before(names[i], names[j]) })
	return names
}

func subDirs(bookDir, dir string) []string {
	entries, _ := os.ReadDir(filepath.Join(bookDir, filepath.FromSlash(dir)))
	var names []string
	for _, e := range entries {
		if e.IsDir() && !skipName(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return before(names[i], names[j]) })
	return names
}

// chaptersIn lists the numbered chapters of a new folder and its numbered
// subfolders, in book order, and reports the unnumbered ones.
func chaptersIn(bookDir, dir string, notAdded *[]NotAdded) []string {
	var files []string
	for _, name := range mdFiles(bookDir, dir) {
		if _, ok := order(name); ok {
			files = append(files, path.Join(dir, name))
		} else {
			*notAdded = append(*notAdded, NotAdded{path.Join(dir, name), "its name has no number, so Octavo cannot tell where it goes"})
		}
	}
	for _, name := range subDirs(bookDir, dir) {
		if _, ok := order(name); ok {
			files = append(files, chaptersIn(bookDir, path.Join(dir, name), notAdded)...)
		}
	}
	return files
}

// newGroup makes the part or section for a new folder: its index.md and
// numbered files, then its numbered subfolders as nested sections.
func newGroup(bookDir, dir string, files []string) config.Entry {
	g := config.Entry{Group: folderTitle(bookDir, dir)}
	subs := map[string][]string{}
	var subOrder []string
	for _, f := range files {
		if path.Dir(f) == dir {
			g.Contents = append(g.Contents, config.Entry{File: f})
			continue
		}
		sub := path.Join(dir, strings.SplitN(strings.TrimPrefix(f, dir+"/"), "/", 2)[0])
		if subs[sub] == nil {
			subOrder = append(subOrder, sub)
		}
		subs[sub] = append(subs[sub], f)
	}
	for _, sub := range subOrder {
		g.Contents = append(g.Contents, newGroup(bookDir, sub, subs[sub]))
	}
	return g
}

// folderTitle is the title of a new folder's part: the title of its index.md
// when it has one, else its name without the number, in title case:
// "17-modern-tools" -> "Modern Tools".
func folderTitle(bookDir, dir string) string {
	if data, err := os.ReadFile(filepath.Join(bookDir, filepath.FromSlash(dir), "index.md")); err == nil {
		if t := quickTitle(string(data)); t != "" {
			return t
		}
	}
	name := strings.TrimLeft(numbered.ReplaceAllString(path.Base(dir), ""), "-_. ")
	small := map[string]bool{"and": true, "or": true, "to": true, "of": true, "for": true, "in": true, "on": true, "the": true, "a": true}
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		if i > 0 && small[strings.ToLower(w)] {
			words[i] = strings.ToLower(w)
			continue
		}
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	if len(words) == 0 {
		return path.Base(dir)
	}
	return strings.Join(words, " ")
}

// quickTitle finds a Markdown file's title: "title:" in its front matter, or
// its first "# " heading.
func quickTitle(src string) string {
	lines := strings.Split(src, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for _, l := range lines[1:] {
			if strings.TrimSpace(l) == "---" {
				break
			}
			if v, ok := strings.CutPrefix(l, "title:"); ok {
				return strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
	}
	for _, l := range lines {
		if t, ok := strings.CutPrefix(l, "# "); ok {
			return strings.TrimSpace(t)
		}
	}
	return ""
}

// spot is an entry's position: the list that holds it, and its index there.
type spot struct {
	list *[]config.Entry
	i    int
}

// chapterSpots finds every chapter entry whose file is in dir.
func chapterSpots(list *[]config.Entry, dir string, out *[]spot) {
	for i := range *list {
		e := &(*list)[i]
		if e.Group != "" {
			chapterSpots(&e.Contents, dir, out)
		} else if path.Dir(filepath.ToSlash(e.File)) == dir {
			*out = append(*out, spot{list, i})
		}
	}
}

// insertChapter puts rel among the chapters of its folder: after the one
// with the next lower number, or before the first when it has the lowest.
func insertChapter(entries *[]config.Entry, dir, rel string) bool {
	var spots []spot
	chapterSpots(entries, dir, &spots)
	name := path.Base(rel)
	var after, first *spot
	for k := range spots {
		s := &spots[k]
		other := path.Base(filepath.ToSlash((*s.list)[s.i].File))
		if _, ok := order(other); !ok {
			continue
		}
		if before(other, name) {
			after = s
		} else if first == nil {
			first = s
		}
	}
	switch {
	case after != nil:
		insertAt(after.list, after.i+1, config.Entry{File: rel})
	case first != nil:
		insertAt(first.list, first.i, config.Entry{File: rel})
	default:
		return false
	}
	return true
}

// groupSpots finds every group whose chapters all come from one folder
// inside parent: the part or section made for that folder.
func groupSpots(list *[]config.Entry, parent string, out map[string]spot) {
	for i := range *list {
		e := &(*list)[i]
		if e.Group == "" {
			continue
		}
		if dir := groupFolder(e, parent); dir != "" {
			if _, seen := out[dir]; !seen {
				out[dir] = spot{list, i}
			}
			continue
		}
		groupSpots(&e.Contents, parent, out)
	}
}

// groupFolder returns the folder directly inside parent that holds every
// chapter of group e, or "" when there is no single one.
func groupFolder(e *config.Entry, parent string) string {
	dir := ""
	var walk func(es []config.Entry) bool
	walk = func(es []config.Entry) bool {
		for i := range es {
			if es[i].Group != "" {
				if !walk(es[i].Contents) {
					return false
				}
				continue
			}
			f := filepath.ToSlash(es[i].File)
			rest, ok := strings.CutPrefix(f, parent+"/")
			if parent == "." {
				rest, ok = f, true
			}
			if !ok || !strings.Contains(rest, "/") {
				return false
			}
			d := path.Join(parent, strings.SplitN(rest, "/", 2)[0])
			if dir != "" && d != dir {
				return false
			}
			dir = d
		}
		return true
	}
	if !walk(e.Contents) {
		return ""
	}
	return dir
}

// insertGroup puts a new folder's group beside the groups of its sibling
// folders, by number.
func insertGroup(entries *[]config.Entry, parent, dir string, g config.Entry) bool {
	groups := map[string]spot{}
	groupSpots(entries, parent, groups)
	name := path.Base(dir)
	var after, first *spot
	var afterName, firstName string
	for d, s := range groups {
		other := path.Base(d)
		if _, ok := order(other); !ok {
			continue
		}
		s := s
		if before(other, name) {
			if after == nil || before(afterName, other) {
				after, afterName = &s, other
			}
		} else if first == nil || before(other, firstName) {
			first, firstName = &s, other
		}
	}
	switch {
	case after != nil:
		insertAt(after.list, after.i+1, g)
	case first != nil:
		insertAt(first.list, first.i, g)
	default:
		return false
	}
	return true
}

func insertAt(list *[]config.Entry, i int, e config.Entry) {
	*list = append(*list, config.Entry{})
	copy((*list)[i+1:], (*list)[i:])
	(*list)[i] = e
}
