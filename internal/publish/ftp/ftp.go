// Package ftp uploads a book to a web host over FTP or FTPS.
package ftp

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"gopkg.in/yaml.v3"

	"github.com/kanthi/octavo/internal/config"
	"github.com/kanthi/octavo/internal/publish"
)

type Settings struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	User        string `yaml:"user"`
	PasswordEnv string `yaml:"password-env"` // name of the environment variable holding the password
	Path        string `yaml:"path"`         // folder on the server, such as public_html/book
	TLS         string `yaml:"tls"`          // explicit (default), implicit or none
}

type publisher struct{ s Settings }

func New(node yaml.Node) (publish.Publisher, error) {
	s := Settings{Port: 21, PasswordEnv: "OCTAVO_FTP_PASSWORD", TLS: "explicit"}
	if err := config.Decode(node, &s); err != nil {
		return nil, err
	}
	if s.Host == "" || s.User == "" {
		return nil, fmt.Errorf("host and user are required")
	}
	switch s.TLS {
	case "explicit", "implicit", "none":
	default:
		return nil, fmt.Errorf("tls must be explicit, implicit or none, not %q", s.TLS)
	}
	return &publisher{s}, nil
}

// Publish uploads every file, overwriting what is there. It does not delete
// files on the server that are no longer part of the book.
func (p *publisher) Publish(ctx context.Context, site publish.Site) error {
	password, ok := os.LookupEnv(p.s.PasswordEnv)
	if !ok {
		return fmt.Errorf("set the FTP password in the %s environment variable", p.s.PasswordEnv)
	}
	options := []ftp.DialOption{ftp.DialWithContext(ctx), ftp.DialWithTimeout(20 * time.Second)}
	switch p.s.TLS {
	case "explicit":
		options = append(options, ftp.DialWithExplicitTLS(&tls.Config{ServerName: p.s.Host}))
	case "implicit":
		options = append(options, ftp.DialWithTLS(&tls.Config{ServerName: p.s.Host}))
	}
	conn, err := ftp.Dial(net.JoinHostPort(p.s.Host, strconv.Itoa(p.s.Port)), options...)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", p.s.Host, err)
	}
	defer conn.Quit()
	if err := conn.Login(p.s.User, password); err != nil {
		return fmt.Errorf("log in as %s: %w", p.s.User, err)
	}

	// Create the target folder one level at a time. Errors are ignored here
	// because a folder that already exists also reports one.
	base := strings.Trim(p.s.Path, "/")
	if strings.HasPrefix(p.s.Path, "/") {
		base = "/" + base
	}
	for i, r := range base {
		if r == '/' && i > 0 {
			_ = conn.MakeDir(base[:i])
		}
	}
	if base != "" && base != "/" {
		_ = conn.MakeDir(base)
	}

	count := 0
	err = filepath.WalkDir(site.Dir, func(local string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(site.Dir, local)
		if err != nil || rel == "." {
			return err
		}
		remote := path.Join(base, filepath.ToSlash(rel))
		if d.IsDir() {
			_ = conn.MakeDir(remote)
			return nil
		}
		file, err := os.Open(local)
		if err != nil {
			return err
		}
		defer file.Close()
		if err := conn.Stor(remote, file); err != nil {
			return fmt.Errorf("upload %s: %w", remote, err)
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(site.Log, "Uploaded %d files to %s\n", count, p.s.Host)
	return nil
}
