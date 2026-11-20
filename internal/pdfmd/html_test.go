package pdfmd

import (
	"os"
	"strings"
	"testing"
)

// TestHTMLKeepsTheStructure: the tree is the reason a PDF can become HTML at
// all - a heading as a heading, a list as a list, a table with its head row.
func TestHTMLKeepsTheStructure(t *testing.T) {
	doc, text := structBook()
	body := htmlTree(doc, text, "")
	for _, want := range []string{
		"<h1>Kapitel eins</h1>",
		"<p>Der erste Absatz.</p>",
		"<ul>",
		"<li>Erster Punkt</li>",
		"<thead><tr><th>Name</th><th>Wert</th></tr></thead>",
		"<tr><td>Alpha</td><td>1</td></tr>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
	// A list must not also come out as loose paragraphs.
	if strings.Contains(body, "<p>Erster Punkt</p>") {
		t.Errorf("the list item was written twice:\n%s", body)
	}
}

// TestHTMLTableHeadIsAHead: a data row written as a header row would make the
// value look like a column name.
func TestHTMLTableHeadIsAHead(t *testing.T) {
	doc, text := structBook()
	body := htmlTree(doc, text, "")
	if strings.Count(body, "<th>") != 2 {
		t.Errorf("want two header cells:\n%s", body)
	}
	if !strings.Contains(body, "<tbody>") {
		t.Errorf("no body section:\n%s", body)
	}
}

// TestHTMLOfATaggedFile is the end to end case on our own output.
func TestHTMLOfATaggedFile(t *testing.T) {
	data, err := os.ReadFile("testdata/tagged.pdf")
	if err != nil {
		t.Skipf("no fixture: %v", err)
	}
	page, err := HTML(data)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		"<!DOCTYPE html>", `<html lang="de">`, "<title>Kapitel eins</title>",
		"<h1>Kapitel eins</h1>", "<h2>Unterkapitel</h2>",
		"<li>Erster Punkt</li>", "<th>Name</th>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "## ") || strings.Contains(page, "| Name |") {
		t.Errorf("markdown leaked into the page:\n%s", page)
	}
}

// TestHTMLOfANonPDFIsAnError: a file that is not a PDF is refused, not read as
// an empty page.
func TestHTMLOfANonPDFIsAnError(t *testing.T) {
	if _, err := HTML([]byte("hello")); err == nil {
		t.Errorf("a text file was accepted as a PDF")
	}
}
