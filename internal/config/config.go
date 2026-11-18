// Package config reads and validates octavo.yml.
//
// The format and publish sections are kept as raw YAML nodes. Each format or
// publish module decodes its own node, so adding a module never changes this
// package.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the book settings file at the root of a book folder.
const FileName = "octavo.yml"

type Config struct {
	Book      Book                 `yaml:"book"`
	Variables map[string]any       `yaml:"variables"` // values for {{< var name >}} in chapters
	OutputDir string               `yaml:"output-dir"`
	Format    map[string]yaml.Node `yaml:"format"`
	Publish   map[string]yaml.Node `yaml:"publish"`
	Versions  []Version            `yaml:"versions"` // editions built side by side, each in its own folder
}

// Version is one edition of the book that "octavo render" builds, from the
// book as it is now or as it was at a git branch, tag or commit.
type Version struct {
	Label  string `yaml:"label"`  // shown in the version menu, such as "2nd edition"
	Ref    string `yaml:"ref"`    // git branch, tag or commit; empty means the book as it is now
	Folder string `yaml:"folder"` // its folder in the output; default: made from the label
}

type Book struct {
	Title       string  `yaml:"title"`
	Author      string  `yaml:"author"`
	Date        string  `yaml:"date"`
	Description string  `yaml:"description"`
	Lang        string  `yaml:"lang"`
	Draft       bool    `yaml:"draft"`    // left out of every build unless --drafts is given
	Created     string  `yaml:"created"`  // when the book was started; default: its first git commit
	SiteURL     string  `yaml:"site-url"` // the address the website will have, for the sitemap and link previews
	Chapters    []Entry `yaml:"chapters"`
	AutoAdd     *bool   `yaml:"auto-add"` // add numbered chapter files that the list leaves out, default true
}

// Entry is one item in the chapter list. It is either a chapter file or a
// group of entries. In YAML it can be written three ways:
//
//   - intro.md                         a chapter
//   - {text: Intro, file: intro.md}    a chapter with its own sidebar label
//   - {part: Basics, contents: [...]}  a group ("part" and "section" both work)
type Entry struct {
	File     string  // chapter path; empty for a group
	Text     string  // sidebar label; empty means use the chapter title
	Group    string  // group title; empty for a chapter
	Contents []Entry // the group's entries
}

func (e *Entry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		e.File = node.Value
		return nil
	}
	var raw struct {
		File     string  `yaml:"file"`
		Text     string  `yaml:"text"`
		Part     string  `yaml:"part"`
		Section  string  `yaml:"section"`
		Contents []Entry `yaml:"contents"`
		Chapters []Entry `yaml:"chapters"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	e.File, e.Text, e.Group = raw.File, raw.Text, raw.Part+raw.Section
	e.Contents = append(raw.Contents, raw.Chapters...)
	if (e.File == "") == (e.Group == "") {
		return fmt.Errorf("line %d: a chapter entry needs either \"file\" or \"part\"/\"section\"", node.Line)
	}
	return nil
}

// Files lists every chapter file in reading order, groups flattened.
func (b Book) Files() []string {
	var files []string
	var walk func([]Entry)
	walk = func(entries []Entry) {
		for _, e := range entries {
			if e.Group == "" {
				files = append(files, e.File)
			}
			walk(e.Contents)
		}
	}
	walk(b.Chapters)
	return files
}

// Load reads octavo.yml from the book folder dir.
func Load(dir string) (*Config, error) {
	file := filepath.Join(dir, FileName)
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no %s in %s (run \"octavo new <dir>\" to create a book)", FileName, dir)
		}
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if c.OutputDir == "" {
		c.OutputDir = "_book"
	}
	// The address can come from the environment, as when GitHub Actions
	// knows it and the book's settings do not.
	if u := os.Getenv("OCTAVO_SITE_URL"); u != "" {
		c.Book.SiteURL = u
	}
	if c.Book.Lang == "" {
		c.Book.Lang = "en"
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &c, nil
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.Book.Title) == "" {
		return fmt.Errorf("book.title is required")
	}
	files := c.Book.Files()
	if len(files) == 0 {
		return fmt.Errorf("book.chapters needs at least one Markdown file")
	}
	if err := c.validateVersions(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, ch := range files {
		switch {
		case !filepath.IsLocal(ch):
			return fmt.Errorf("chapter %q must be a path inside the book folder", ch)
		case !strings.HasSuffix(ch, ".md"):
			return fmt.Errorf("chapter %q must be a .md file", ch)
		case seen[filepath.ToSlash(ch)]:
			return fmt.Errorf("chapter %q is listed twice", ch)
		}
		seen[filepath.ToSlash(ch)] = true
	}
	return nil
}

func (c *Config) validateVersions() error {
	folders, current := map[string]bool{}, 0
	for i := range c.Versions {
		v := &c.Versions[i]
		if strings.TrimSpace(v.Label) == "" {
			return fmt.Errorf("versions: entry %d needs a label", i+1)
		}
		if v.Folder == "" {
			v.Folder = Slug(v.Label)
		}
		if v.Folder == "" || strings.ContainsAny(v.Folder, `/\`) || v.Folder == "." || v.Folder == ".." || v.Folder == "assets" {
			return fmt.Errorf("versions: %q: folder %q must be one plain folder name", v.Label, v.Folder)
		}
		if folders[v.Folder] {
			return fmt.Errorf("versions: two versions use the folder %q; give one a \"folder:\" of its own", v.Folder)
		}
		folders[v.Folder] = true
		if v.Ref == "" {
			current++
		}
	}
	if current > 1 {
		return fmt.Errorf("versions: only one version can be the book as it is now (no \"ref:\"); give the others a git branch or tag")
	}
	return nil
}

// Slug makes a folder name from a label: "2nd Edition (2026)" -> "2nd-edition-2026".
func Slug(label string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
	}
	return strings.Trim(b.String(), ".")
}

// Decode fills v from a module's settings node. A node that is not a mapping
// (for example "html: true" or an empty "gh-pages:") leaves v at its defaults.
func Decode(node yaml.Node, v any) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	return node.Decode(v)
}

// Enabled reports whether a module is switched on. Only an explicit "false"
// switches it off.
func Enabled(node yaml.Node) bool {
	return !(node.Kind == yaml.ScalarNode && node.Value == "false")
}

// StringList accepts either one string or a list of strings in YAML.
type StringList []string

func (l *StringList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*l = StringList{node.Value}
		return nil
	}
	var items []string
	if err := node.Decode(&items); err != nil {
		return err
	}
	*l = items
	return nil
}
