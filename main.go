// txt2epub converts a UTF-8 plain-text file into a simple EPUB 3 book.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const usage = `Usage: txt2epub -input book.txt [options]

Options:
  -input   Source text file (UTF-8; required)
  -output  EPUB destination (default: source filename with .epub)
  -title   Book title (default: source filename)
  -author  Author name (default: Unknown)
  -lang    Book language (default: ja)

Blank lines separate paragraphs. Lines beginning with "## " create chapter headings.
`

type bookOptions struct {
	title, author, lang string
}

type chapter struct {
	Title string
	Body  template.HTML
}

// brTag matches common HTML line-break tag spellings in source text.
var brTag = regexp.MustCompile(`(?i)<br\s*/?>`)

func main() {

	input := flag.String("input", "", "source text file (required)")
	output := flag.String("output", "", "EPUB destination")
	title := flag.String("title", "", "book title")
	author := flag.String("author", "Unknown", "author name")
	lang := flag.String("lang", "ja", "book language")

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
	}
	// 実行時に渡されたコマンドライン引数を読み取って、flag.String などで用意した変数へ反映する処理です。
	flag.Parse()

	if *input == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *output == "" {
		*output = strings.TrimSuffix(*input, filepath.Ext(*input)) + ".epub"
	}
	if *title == "" {
		*title = strings.TrimSuffix(filepath.Base(*input), filepath.Ext(*input))
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fail(err)
	}
	if err := writeEPUB(*output, string(data), bookOptions{*title, *author, *lang}); err != nil {
		fail(err)
	}
	fmt.Printf("Created %s\n", *output)
}

func fail(err error) { fmt.Fprintln(os.Stderr, "txt2epub:", err); os.Exit(1) }

func writeEPUB(output, source string, opt bookOptions) error {
	chapters := parseChapters(source)
	if len(chapters) == 0 {
		chapters = []chapter{{Title: opt.title, Body: "<p></p>"}}
	}

	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	zw := zip.NewWriter(file)
	defer zw.Close()

	// EPUB requires this to be first and uncompressed.
	if err := addZipFile(zw, "mimetype", "application/epub+zip", zip.Store); err != nil {
		return err
	}
	if err := addZipFile(zw, "META-INF/container.xml", containerXML, zip.Deflate); err != nil {
		return err
	}

	id := bookID(opt.title, opt.author)
	if err := addZipFile(zw, "OEBPS/content.opf", contentOPF(opt, id, len(chapters)), zip.Deflate); err != nil {
		return err
	}
	if err := addZipFile(zw, "OEBPS/nav.xhtml", navXHTML(opt, chapters), zip.Deflate); err != nil {
		return err
	}
	for i, c := range chapters {
		if err := addZipFile(zw, fmt.Sprintf("OEBPS/chapter-%d.xhtml", i+1), chapterXHTML(opt, c), zip.Deflate); err != nil {
			return err
		}
	}
	return zw.Close()
}

func addZipFile(zw *zip.Writer, name, content string, method uint16) error {
	h := &zip.FileHeader{Name: name, Method: method}
	h.SetModTime(time.Now())
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, content)
	return err
}

func parseChapters(text string) []chapter {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = brTag.ReplaceAllString(text, "")
	var result []chapter
	currentTitle := "本文"
	var lines []string
	flush := func(includeEmpty bool) {
		body := paragraphs(strings.Join(lines, "\n"))
		if body != "" || includeEmpty {
			result = append(result, chapter{Title: currentTitle, Body: template.HTML(body)})
		}
	}
	sawHeading := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			// Do not emit an empty default chapter before the first heading.
			flush(sawHeading)
			currentTitle = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			lines = nil
			sawHeading = true
			continue
		}
		lines = append(lines, line)
	}
	flush(sawHeading)
	return result
}

func paragraphs(text string) string {
	var out []string
	for _, p := range strings.Split(strings.TrimSpace(text), "\n\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, "<p>"+template.HTMLEscapeString(p)+"</p>")
		}
	}
	return strings.Join(out, "\n")
}

func bookID(title, author string) string {
	sum := sha1.Sum([]byte(title + "\x00" + author))
	return "urn:sha1:" + hex.EncodeToString(sum[:])
}

const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`

func contentOPF(opt bookOptions, id string, n int) string {
	var manifest, spine strings.Builder
	manifest.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&manifest, `<item id="c%d" href="chapter-%d.xhtml" media-type="application/xhtml+xml"/>`, i, i)
		fmt.Fprintf(&spine, `<itemref idref="c%d"/>`, i)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id" xml:lang="%s"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">%s</dc:identifier><dc:title>%s</dc:title><dc:creator>%s</dc:creator><dc:language>%s</dc:language><meta property="dcterms:modified">%s</meta></metadata><manifest>%s</manifest><spine>%s</spine></package>`, esc(opt.lang), esc(id), esc(opt.title), esc(opt.author), esc(opt.lang), time.Now().UTC().Format("2006-01-02T15:04:05Z"), manifest.String(), spine.String())
}

func navXHTML(opt bookOptions, chapters []chapter) string {
	var items strings.Builder
	for i, c := range chapters {
		fmt.Fprintf(&items, `<li><a href="chapter-%d.xhtml">%s</a></li>`, i+1, esc(c.Title))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s"><head><title>%s</title></head><body><nav epub:type="toc" xmlns:epub="http://www.idpf.org/2007/ops"><h1>目次</h1><ol>%s</ol></nav></body></html>`, esc(opt.lang), esc(opt.title), items.String())
}

func chapterXHTML(opt bookOptions, c chapter) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s"><head><title>%s</title><meta charset="utf-8"/></head><body><h1>%s</h1>%s</body></html>`, esc(opt.lang), esc(c.Title), esc(c.Title), c.Body)
}

func esc(s string) string { var b bytes.Buffer; template.HTMLEscape(&b, []byte(s)); return b.String() }
