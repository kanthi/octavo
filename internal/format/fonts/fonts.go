// Package fonts holds the fonts bundled with Octavo: the reading fonts
// Literata, Source Serif 4 and Atkinson Hyperlegible Next, and JetBrains Mono
// for code. They are under the SIL Open Font License and ship inside the
// binary, so that books never load a font from another site.
//
// Each reading font is a variable WOFF2 file per style (normal, italic) and
// per set of letters (Latin, Latin Extended). The code font is one file per
// style that also holds the symbols code uses: arrows, box drawing, blocks
// and shapes. Other scripts fall back to the reader's own fonts.
package fonts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/font"
)

//go:embed files
var files embed.FS

// Font is one bundled typeface.
type Font struct {
	Key    string // the name used in octavo.yml: literata, source-serif, atkinson
	Family string // the CSS font-family name
	slug   string // file name prefix
	weight string // the range of weights the variable font covers
	single bool   // one file per style, not split into Latin and Latin Extended
}

var All = []Font{
	{"literata", "Literata", "literata", "200 900", false},
	{"source-serif", "Source Serif 4", "source-serif-4", "200 900", false},
	{"atkinson", "Atkinson Hyperlegible Next", "atkinson-hyperlegible-next", "200 800", false},
}

// Mono is the font for code. It is not one of the reading fonts in All.
var Mono = Font{"jetbrains-mono", "JetBrains Mono", "jetbrains-mono", "100 800", true}

// Find returns the bundled reading font with that key.
func Find(key string) (Font, bool) {
	for _, f := range All {
		if f.Key == key {
			return f, true
		}
	}
	return Font{}, false
}

// Keys lists the bundled fonts' names.
func Keys() []string {
	keys := make([]string, len(All))
	for i, f := range All {
		keys[i] = f.Key
	}
	return keys
}

var ranges = []struct{ name, unicode string }{
	{"latin-ext", "U+0100-02BA,U+02BD-02C5,U+02C7-02CC,U+02CE-02D7,U+02DD-02FF,U+0304,U+0308,U+0329,U+1D00-1DBF,U+1E00-1E9F,U+1EF2-1EFF,U+2020,U+20A0-20AB,U+20AD-20C0,U+2113,U+2C60-2C7F,U+A720-A7FF"},
	{"latin", "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD"},
}

// CSS returns the @font-face rules for the given fonts. base is put in front
// of every file name, for example "fonts/" or a file:// address.
func CSS(base string, which ...Font) string {
	var s strings.Builder
	for _, f := range which {
		for _, style := range []string{"normal", "italic"} {
			if f.single {
				fmt.Fprintf(&s, "@font-face { font-family: %q; font-style: %s; font-weight: %s; font-display: swap; src: url(%s%s-wght-%s.woff2) format(\"woff2\"); }\n",
					f.Family, style, f.weight, base, f.slug, style)
				continue
			}
			for _, r := range ranges {
				fmt.Fprintf(&s, "@font-face { font-family: %q; font-style: %s; font-weight: %s; font-display: swap; src: url(%s%s-%s-wght-%s.woff2) format(\"woff2\"); unicode-range: %s; }\n",
					f.Family, style, f.weight, base, f.slug, r.name, style, r.unicode)
			}
		}
	}
	return s.String()
}

// Names lists every bundled file: the fonts and their licence texts.
func Names() []string {
	entries, _ := files.ReadDir("files")
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

// Read returns one bundled file.
func Read(name string) []byte {
	data, _ := files.ReadFile("files/" + name)
	return data
}

// WriteTo copies the files of the given fonts into dir.
func WriteTo(dir string, which ...Font) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range Names() {
		for _, f := range which {
			if strings.HasPrefix(name, f.slug+"-") {
				if err := os.WriteFile(filepath.Join(dir, name), Read(name), 0o644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// WriteTTF writes the Latin files of the given fonts into dir as TrueType,
// for programs that cannot read WOFF2 (Typst). The Latin Extended files are
// left out: they carry the same family name, so a program would pick one of
// the two at random. Letters outside Latin fall back to the system's fonts.
func WriteTTF(dir string, which ...Font) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range which {
		for _, style := range []string{"normal", "italic"} {
			name := fmt.Sprintf("%s-latin-wght-%s", f.slug, style)
			if f.single {
				name = fmt.Sprintf("%s-wght-%s", f.slug, style)
			}
			ttf, err := font.ToSFNT(Read(name + ".woff2"))
			if err != nil {
				return fmt.Errorf("font %s: %w", name, err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".ttf"), ttf, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
