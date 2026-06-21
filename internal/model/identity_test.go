package model

import "testing"

func TestGUIDIdentitySurvivesTitleChange(t *testing.T) {
	first := Article{FeedURL: "https://example.com/feed", GUID: "item-1", Title: "Old title", Link: "https://example.com/a"}
	second := Article{FeedURL: "https://example.com/feed", GUID: "item-1", Title: "New title", Link: "https://example.com/a"}
	PrepareArticleIdentity(&first)
	PrepareArticleIdentity(&second)
	if first.ArticleKey != second.ArticleKey {
		t.Fatalf("GUID-stable article key changed")
	}
}

func TestLinkNormalizationTrackingAndMeaningfulQuery(t *testing.T) {
	a := NormalizeArticleLink("HTTPS://Example.COM:443/path?b=2&utm_source=x&a=1#frag")
	b := NormalizeArticleLink("https://example.com/path?a=1&b=2")
	c := NormalizeArticleLink("https://example.com/path?a=2&b=2")
	if a != b {
		t.Fatalf("expected tracking params and fragment removed: %q != %q", a, b)
	}
	if b == c {
		t.Fatalf("meaningful query difference was merged")
	}
}

func TestFallbackIdentityIsFeedScoped(t *testing.T) {
	first := Article{FeedURL: "https://one.example/feed", Title: "Same"}
	second := Article{FeedURL: "https://two.example/feed", Title: "Same"}
	PrepareArticleIdentity(&first)
	PrepareArticleIdentity(&second)
	if first.ArticleKey == second.ArticleKey {
		t.Fatalf("article key must be feed scoped")
	}
}

func TestLegacyHashCompatibility(t *testing.T) {
	got := LegacyHash("Title", "https://example.com/a")
	want := "3e8a27f0d70971ee4d29ec6b8adcd53f912a70b9533acb575b56b6160a114982"
	if got != want {
		t.Fatalf("legacy hash changed: got %s want %s", got, want)
	}
}
