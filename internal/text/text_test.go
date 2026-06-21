package text

import "testing"

func TestCleanHTMLAndWhitespace(t *testing.T) {
	got := CleanHTML("<p>Hello&nbsp;<strong>world</strong></p><p>Next<br>line</p>")
	want := "Hello world\nNext\nline"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTruncateRunesIsUnicodeSafe(t *testing.T) {
	got := TruncateRunes("abc😀def", 6)
	if got != "abc..." {
		t.Fatalf("unexpected truncation %q", got)
	}
}
