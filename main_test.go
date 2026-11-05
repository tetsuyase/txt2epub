package main

import (
	"archive/zip"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteEPUBCreatesRequiredFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	if err := writeEPUB(path, "前書き\n\n本文\n## 第一章\n章の内容", bookOptions{"テスト", "著者", "ja"}); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.File) < 5 || r.File[0].Name != "mimetype" || r.File[0].Method != zip.Store {
		t.Fatalf("invalid EPUB mimetype entry")
	}
	seen := map[string]bool{}
	for _, f := range r.File {
		seen[f.Name] = true
	}
	for _, name := range []string{"META-INF/container.xml", "OEBPS/content.opf", "OEBPS/nav.xhtml", "OEBPS/chapter-1.xhtml", "OEBPS/chapter-2.xhtml"} {
		if !seen[name] {
			t.Errorf("missing %s", name)
		}
	}
}

func TestParseChaptersEscapesText(t *testing.T) {
	chapters := parseChapters("<tag> & text\n## Next\nbody")
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters", len(chapters))
	}
	if !strings.Contains(string(chapters[0].Body), "&lt;tag&gt; &amp; text") {
		t.Fatal("body was not escaped")
	}
	if chapters[1].Title != "Next" {
		t.Fatal("heading was not parsed")
	}
}

func TestParseChaptersDoesNotAddEmptyIntroBeforeHeading(t *testing.T) {
	chapters := parseChapters("## 第一章\n内容")
	if len(chapters) != 1 || chapters[0].Title != "第一章" {
		t.Fatalf("unexpected chapters: %#v", chapters)
	}
}

func TestParseChaptersRemovesBRTags(t *testing.T) {
	chapters := parseChapters("一行目<br/>二行目<br>三行目<br />四行目\n五行目")
	if len(chapters) != 1 {
		t.Fatalf("got %d chapters", len(chapters))
	}
	body := string(chapters[0].Body)
	if strings.Contains(strings.ToLower(body), "br") || !strings.Contains(body, "一行目二行目三行目四行目\n五行目") {
		t.Fatalf("BR tags were not removed: %s", body)
	}
}
