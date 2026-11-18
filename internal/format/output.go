package format

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/kanthi/octavo/internal/fsutil"
)

// Output is the folder a book is rendered into. It remembers a fingerprint
// for every file it wrote, which is what makes rebuilds incremental: a format
// describes the inputs of each file as a fingerprint, and Output skips the
// file when that fingerprint is the same as last time.
//
// A new Output has no memory, so its first build writes everything. Keep the
// same Output between builds (as the preview does) to rebuild only what
// changed. Files that a build no longer produces are deleted.
type Output struct {
	Dir string

	files   map[string]uint64 // output path -> fingerprint, from earlier builds
	seen    map[string]bool   // output paths this build still wants
	changed []string          // output paths this build wrote
}

func NewOutput(dir string) *Output {
	return &Output{Dir: dir, files: map[string]uint64{}}
}

// Hash folds any values into one fingerprint.
func Hash(parts ...any) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		switch v := p.(type) {
		case []byte:
			h.Write(v)
		case string:
			io.WriteString(h, v)
		default:
			fmt.Fprint(h, v)
		}
		h.Write([]byte{0})
	}
	return h.Sum64()
}

// Fresh reports whether rel was already written from the same inputs. When it
// returns true the caller can skip the work of producing the file.
func (o *Output) Fresh(rel string, fingerprint uint64) bool {
	o.seen[rel] = true
	old, ok := o.files[rel]
	return ok && old == fingerprint
}

// Write stores data at rel and records the fingerprint of its inputs.
func (o *Output) Write(rel string, fingerprint uint64, data []byte) error {
	if err := fsutil.WriteFile(filepath.Join(o.Dir, filepath.FromSlash(rel)), data); err != nil {
		return err
	}
	o.record(rel, fingerprint)
	return nil
}

// WriteData stores data at rel unless the same bytes are already there.
func (o *Output) WriteData(rel string, data []byte) error {
	fp := Hash(data)
	if o.Fresh(rel, fp) {
		return nil
	}
	return o.Write(rel, fp, data)
}

// Copy copies the file src to rel unless src is unchanged since the last copy.
func (o *Output) Copy(src, rel string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	fp := Hash(src, info.ModTime().UnixNano(), info.Size())
	if o.Fresh(rel, fp) {
		return nil
	}
	if err := fsutil.CopyFile(src, filepath.Join(o.Dir, filepath.FromSlash(rel))); err != nil {
		return err
	}
	o.record(rel, fp)
	return nil
}

func (o *Output) record(rel string, fingerprint uint64) {
	o.seen[rel] = true
	o.files[rel] = fingerprint
	o.changed = append(o.changed, rel)
}

// Changed lists the files the last build wrote, in alphabetical order.
func (o *Output) Changed() []string {
	sort.Strings(o.changed)
	return o.changed
}

// Total is the number of files in the output.
func (o *Output) Total() int { return len(o.files) }

// begin starts a build. The first build empties the folder, unless keep is
// set, which is how a build of some formats leaves the others' files alone.
func (o *Output) begin(keep bool) error {
	o.seen, o.changed = map[string]bool{}, nil
	if len(o.files) == 0 && !keep {
		return fsutil.EmptyDir(o.Dir)
	}
	return os.MkdirAll(o.Dir, 0o755)
}

// end finishes a build by deleting files that it did not ask for.
func (o *Output) end() error {
	for rel := range o.files {
		if o.seen[rel] {
			continue
		}
		delete(o.files, rel)
		if err := os.Remove(filepath.Join(o.Dir, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
