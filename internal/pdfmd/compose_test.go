package pdfmd

import (
	"strings"
	"testing"
)

// line builds a text line from pieces at the given x positions.
func line(y, size float64, cols ...string) textLine {
	l := textLine{y: y, size: size}
	x := 30.0
	for _, c := range cols {
		l.pieces = append(l.pieces, piece{text: c, x: x, y: y, w: float64(len(c)) * size * 0.5})
		x += 200
	}
	l.text = strings.Join(cols, " ")
	return l
}

// TestPageWithTableKeepsTheTextAroundIt is the regression for the page that
// lost everything but its table: a heading, a paragraph and a trailing
// paragraph stood on the same page as a table and were gone after reading.
func TestPageWithTableKeepsTheTextAroundIt(t *testing.T) {
	lines := []textLine{
		line(773, 14, "Nur Text"),
		line(740, 10, "Ein Absatz danach."),
		line(710, 10, "A", "B"),
		line(696, 10, "1", "2"),
		line(682, 10, "3", "4"),
		line(638, 10, "Schlussabsatz hier."),
	}
	// Cells drawn for two of the three rows, which is what the writer we read
	// with produces: the row without cells is still part of the table by text.
	rects := []rect{
		{x: 28, y: 705, w: 29, h: 14}, {x: 228, y: 705, w: 29, h: 14},
		{x: 28, y: 677, w: 29, h: 14}, {x: 228, y: 677, w: 29, h: 14},
	}
	got := composePage(lines, rects)

	for _, want := range []string{"## Nur Text", "Ein Absatz danach.", "| A | B |", "| 1 | 2 |", "| 3 | 4 |", "Schlussabsatz hier."} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "| A | B |") != 1 {
		t.Errorf("the table is printed twice:\n%s", got)
	}
}

// TestCellsDecideWhenTheTextDoesNotLineUp: a table whose cells are drawn but
// whose text drifts out of columns is still a table, and the text outside the
// cells survives it. The rows here shift a few points per row, so the text path
// finds no columns and the drawn cells have to speak.
func TestCellsDecideWhenTheTextDoesNotLineUp(t *testing.T) {
	drift := func(y, size float64, xs []float64, cols ...string) textLine {
		l := textLine{y: y, size: size}
		for i, c := range cols {
			l.pieces = append(l.pieces, piece{text: c, x: xs[i], y: y, w: 30})
		}
		l.text = strings.Join(cols, " ")
		return l
	}
	lines := []textLine{
		line(773, 14, "Kopfzeile"),
		drift(710, 10, []float64{20, 120}, "A", "B"),
		drift(690, 10, []float64{26, 133}, "eins", "zwei"),
		drift(670, 10, []float64{31, 141}, "drei", "vier"),
		line(600, 10, "Fusszeile"),
	}
	rects := []rect{
		{x: 18, y: 704, w: 62, h: 16}, {x: 118, y: 704, w: 62, h: 16},
		{x: 18, y: 684, w: 62, h: 16}, {x: 118, y: 684, w: 62, h: 16},
		{x: 18, y: 664, w: 62, h: 16}, {x: 118, y: 664, w: 62, h: 16},
	}
	got := composePage(lines, rects)
	for _, want := range []string{"Kopfzeile", "| A | B |", "| eins | zwei |", "| drei | vier |", "Fusszeile"} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks %q:\n%s", want, got)
		}
	}
}

// TestPageWithoutTableIsUnchanged: the ordinary page must not pay for the fix.
func TestPageWithoutTableIsUnchanged(t *testing.T) {
	lines := []textLine{
		line(773, 14, "Titel"),
		line(740, 10, "Ein Absatz."),
	}
	if got, want := composePage(lines, nil), "### Titel\n\nEin Absatz.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
