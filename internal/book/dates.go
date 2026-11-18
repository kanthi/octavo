package book

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dates are days written as YYYY-MM-DD, so that they sort as text.
//
// A file's date is the day of the last git commit that changed it, when the
// book is in a git repository and the file has no changes since. Otherwise it
// is the day the file was last saved. File times alone are not trusted for
// committed files, because a fresh clone gives every file today's time.

// gitLog is what git says about a book folder. It only changes with a commit,
// so the cache keeps it until HEAD moves.
type gitLog struct {
	head  string
	last  map[string]string // file (relative to the book) -> day of its last commit
	first string            // day of the first commit that touched the book
}

// gitState is gitLog plus the files changed since the last commit.
type gitState struct {
	*gitLog
	changed map[string]bool
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stderr = nil
	return cmd.Output()
}

// readGit returns nil when dir is not in a git repository, or git is missing.
// prev is reused when HEAD has not moved.
func readGit(dir string, prev *gitLog) *gitState {
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	head, err := git(dir, "rev-parse", "HEAD")
	if err != nil {
		return nil // not a repository, or one with no commits yet
	}
	log := prev
	if log == nil || log.head != string(head) {
		out, err := git(dir, "log", "--format=%x00%cs", "--name-only", "--relative", "--", ".")
		if err != nil {
			return nil
		}
		log = &gitLog{head: string(head), last: map[string]string{}}
		for _, commit := range bytes.Split(out, []byte{0}) {
			lines := strings.Split(strings.TrimSpace(string(commit)), "\n")
			if len(lines) == 0 || lines[0] == "" {
				continue
			}
			day := lines[0]
			log.first = day // the log is newest first, so the last one seen is the first commit
			for _, f := range lines[1:] {
				if f = strings.TrimSpace(f); f != "" {
					if _, seen := log.last[f]; !seen {
						log.last[f] = day
					}
				}
			}
		}
	}
	st := &gitState{gitLog: log, changed: map[string]bool{}}
	for _, args := range [][]string{
		{"diff", "--name-only", "--relative", "HEAD", "--", "."},
		{"ls-files", "--others", "--exclude-standard", "--", "."},
	} {
		out, err := git(dir, args...)
		if err != nil {
			return nil
		}
		for _, f := range strings.Split(string(out), "\n") {
			if f = strings.TrimSpace(f); f != "" {
				st.changed[f] = true
			}
		}
	}
	return st
}

// day returns the date of one file of the book (rel uses forward slashes).
func (g *gitState) day(bookDir, rel string) string {
	if g != nil && !g.changed[rel] {
		if d, ok := g.last[rel]; ok {
			return d
		}
	}
	info, err := os.Stat(filepath.Join(bookDir, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return info.ModTime().Format("2006-01-02")
}
