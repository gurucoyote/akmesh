package htmlmd

import "testing"

func TestConvertGFMWritesASeparator(t *testing.T) {
	got := ConvertGFM("<table><thead><tr><th>A</th><th>B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>")
	want := "A | B\n--- | ---\n1 | 2\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if plain := Convert("<table><tr><td>a</td><td>b</td></tr></table>"); plain != "a | b\n" {
		t.Errorf("plain form changed: %q", plain)
	}
}
