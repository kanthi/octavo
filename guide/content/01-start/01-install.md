# Install

Octavo is one program, `octavo`, with nothing else to install for websites and EPUB files. Only the PDF needs something extra: Typst, or a Chromium-family browser.

## With Go

If you have [Go](https://go.dev/dl/) 1.27 or later:

```sh
go install github.com/kanthi/octavo/cmd/octavo@latest
```

The program lands in Go's `bin` folder (usually `~/go/bin`). Make sure that folder is on your `PATH`.

## From the source

```sh
git clone https://github.com/kanthi/octavo.git
cd octavo
go build -o octavo ./cmd/octavo
./octavo version
```

Move the `octavo` file to any folder on your `PATH`, such as `/usr/local/bin` or `~/bin`.

## Check that it works

```sh
octavo version
octavo guide
```

The first prints the version. The second opens this guide in your browser.

## For PDF output: Typst, or a browser

The best PDFs come from [Typst](https://typst.app), a typesetting program, one small download:

```sh
brew install typst          # macOS and Linux with Homebrew
winget install Typst.Typst  # Windows
```

Without Typst, Octavo has a browser print the book instead, without opening a window: **Google Chrome**, **Chromium**, **Microsoft Edge** or **Brave**. Octavo finds either by itself. With neither, it still builds the website and the EPUB and tells you it skipped the PDF.

See [The PDF](../03-formats/04-pdf.md) for the difference between the two, and to choose one.
