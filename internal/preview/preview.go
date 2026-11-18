// Package preview serves a book locally and rebuilds it when a file changes.
package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kanthi/octavo/internal/library"
)

// Run serves a site on addr until ctx is cancelled. build writes the site
// into the folder it is given and returns the files it wrote, relative to
// that folder. It runs once at the start and again whenever a file under
// watchDir changes; a good build function only redoes what changed. Browser
// tabs reload only when the page they show, or a style or image, was written.
// ready is called with the address once the server is listening.
func Run(ctx context.Context, watchDir, addr string, log io.Writer, build func(outDir string) ([]string, error), ready func(url string)) error {
	tmp, err := os.MkdirTemp("", "octavo-preview-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	dir, err := filepath.Abs(watchDir)
	if err != nil {
		return err
	}

	first := true
	rebuild := func() []string {
		start := time.Now()
		changed, err := build(tmp)
		took := time.Since(start).Round(time.Millisecond)
		stamp := time.Now().Format("15:04:05")
		switch {
		case err != nil:
			fmt.Fprintf(log, "%s Build failed: %v\n", stamp, err)
		case first:
			fmt.Fprintf(log, "%s Built %d files in %s\n", stamp, len(changed), took)
		case len(changed) == 0:
			fmt.Fprintf(log, "%s Nothing to rebuild (%s)\n", stamp, took)
		default:
			shown := changed
			more := ""
			if len(shown) > 4 {
				shown, more = shown[:4], fmt.Sprintf(" and %d more", len(changed)-4)
			}
			noun := "files"
			if len(changed) == 1 {
				noun = "file"
			}
			fmt.Fprintf(log, "%s Rebuilt %d %s in %s: %s%s\n", stamp, len(changed), noun, took, strings.Join(shown, ", "), more)
		}
		first = false
		return changed
	}
	rebuild()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	reload := &hub{clients: map[chan string]bool{}}
	files := http.FileServer(http.Dir(tmp))
	mux := http.NewServeMux()
	mux.Handle("/__octavo/reload", reload)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	go func() {
		last := fingerprint(dir)
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if now := fingerprint(dir); now != last {
					last = now
					if changed := rebuild(); len(changed) > 0 {
						reload.broadcast(changed)
					}
				}
			}
		}
	}()

	if ready != nil {
		ready("http://" + listener.Addr().String() + "/")
	}
	if err := server.Serve(listener); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// fingerprint changes whenever a source file is added, removed or edited.
// Folders starting with "." or "_" (such as .git and _book) are skipped, and
// so is a library's "public" folder.
func fingerprint(dir string) uint64 {
	h := fnv.New64a()
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != dir && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || isLibraryOutput(p)) {
				return filepath.SkipDir
			}
			return nil
		}
		if info, err := d.Info(); err == nil {
			fmt.Fprintf(h, "%s|%d|%d\n", p, info.ModTime().UnixNano(), info.Size())
		}
		return nil
	})
	return h.Sum64()
}

// isLibraryOutput reports whether p is the public folder of a library.
func isLibraryOutput(p string) bool {
	if !library.Is(filepath.Dir(p)) {
		return false
	}
	l, err := library.Load(filepath.Dir(p))
	return err == nil && l.PublicDir() == p
}

// hub tells open browser tabs which files changed, over server-sent events.
// Each tab decides for itself whether it needs to reload.
type hub struct {
	mu      sync.Mutex
	clients map[chan string]bool
}

func (h *hub) broadcast(changed []string) {
	data, _ := json.Marshal(changed)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- string(data):
		default:
		}
	}
}

func (h *hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	c := make(chan string, 8)
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-c:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
