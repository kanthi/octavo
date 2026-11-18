package cli

import (
	"github.com/kanthi/octavo/internal/format"
	"github.com/kanthi/octavo/internal/format/epub"
	"github.com/kanthi/octavo/internal/format/html"
	"github.com/kanthi/octavo/internal/format/pdf"
	"github.com/kanthi/octavo/internal/publish"
	"github.com/kanthi/octavo/internal/publish/command"
	"github.com/kanthi/octavo/internal/publish/folder"
	"github.com/kanthi/octavo/internal/publish/ftp"
	"github.com/kanthi/octavo/internal/publish/ghpages"
	"github.com/kanthi/octavo/internal/publish/rsync"
)

// This is the one place that lists every module. A new output format or
// publish target is one new package plus one line here.
func init() {
	format.Register("html", html.New)
	format.Register("epub", epub.New)
	format.Register("pdf", pdf.New)

	publish.Register("gh-pages", ghpages.New)
	publish.Register("ftp", ftp.New)
	publish.Register("rsync", rsync.New)
	publish.Register("folder", folder.New)
	publish.Register("command", command.New)
}
