// Package publish defines what a publish target is and keeps the list of
// available ones. To add a target (Cloudflare, Netlify, S3, ...): write a
// package with a Factory, then register it in internal/cli/modules.go.
package publish

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/config"
)

// Site is a rendered book that is ready to upload.
type Site struct {
	Dir     string    // folder holding the rendered files
	BookDir string    // the book's source folder
	Log     io.Writer // progress messages
}

// Publisher puts a rendered book somewhere people can read it.
type Publisher interface {
	Publish(ctx context.Context, site Site) error
}

// Factory builds a Publisher from its settings under "publish:" in octavo.yml.
type Factory func(settings yaml.Node) (Publisher, error)

var registry = map[string]Factory{}

func Register(kind string, f Factory) { registry[kind] = f }

// Kinds lists the registered target kinds in alphabetical order.
func Kinds() []string {
	kinds := make([]string, 0, len(registry))
	for k := range registry {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// New builds the target called name. Its kind is the "type" key in its
// settings, or the name itself when there is no "type". That lets one book
// have several targets of the same kind under different names.
func New(name string, settings yaml.Node) (Publisher, error) {
	var head struct {
		Type string `yaml:"type"`
	}
	if err := config.Decode(settings, &head); err != nil {
		return nil, fmt.Errorf("publish %s: %w", name, err)
	}
	kind := head.Type
	if kind == "" {
		kind = name
	}
	factory, ok := registry[kind]
	if !ok {
		return nil, fmt.Errorf("publish %s: unknown type %q (available: %s)", name, kind, strings.Join(Kinds(), ", "))
	}
	p, err := factory(settings)
	if err != nil {
		return nil, fmt.Errorf("publish %s: %w", name, err)
	}
	return p, nil
}
