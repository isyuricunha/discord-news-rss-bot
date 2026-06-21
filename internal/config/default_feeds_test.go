package config

import (
	"net/url"
	"testing"
)

func TestDefaultFeedDefinitions(t *testing.T) {
	groups := defaultFeedGroups()
	if len(groups) != 3 {
		t.Fatalf("expected three default groups, got %d", len(groups))
	}

	seen := map[string]struct{}{}
	count := 0
	for _, group := range groups {
		if group.category == "" {
			t.Fatal("default feed group has empty category")
		}
		if categoryEmoji(group.category) == "📢" {
			t.Fatalf("default category lacks a specific emoji: %q", group.category)
		}
		for _, feedURL := range group.urls {
			count++
			parsed, err := url.Parse(feedURL)
			if err != nil {
				t.Fatalf("invalid default URL %q: %v", feedURL, err)
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				t.Fatalf("default URL must use HTTP or HTTPS: %q", feedURL)
			}
			if SourceName(feedURL) == "Source" {
				t.Fatalf("default URL lacks source mapping: %q", feedURL)
			}
			key := parsed.String()
			if _, exists := seen[key]; exists {
				t.Fatalf("duplicate default URL: %q", feedURL)
			}
			seen[key] = struct{}{}
		}
	}

	cfg, err := Load(LoadOptions{RequireWebhook: false, OS: "linux", Env: map[string]string{"RSS_BOT_DATA": t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Feeds) != count {
		t.Fatalf("default feed count should be derived from definitions: got %d want %d", len(cfg.Feeds), count)
	}
}

func TestDefaultFeedRefreshRemovesObsoleteURLs(t *testing.T) {
	obsolete := []string{
		"https://www.band.uol.com.br/rss/noticias.xml",
		"https://feeds.folha.uol.com.br/folha/rss02.xml",
		"https://www.gazetadopovo.com.br/rss/brasil.xml",
		"https://jovempan.com.br/rss.xml",
		"https://www.metropoles.com/rss.xml",
		"https://www.oantagonista.com/rss/",
		"https://www.terra.com.br/rss/politica/",
		"https://www.tecmundo.com.br/rss",
		"https://www.oficinadanet.com.br/rss",
	}
	current := defaultURLSet()
	for _, feedURL := range obsolete {
		if _, exists := current[feedURL]; exists {
			t.Fatalf("obsolete default URL still present: %s", feedURL)
		}
	}
}

func TestDefaultFeedRefreshIncludesVerifiedReplacements(t *testing.T) {
	replacements := []string{
		"https://rss.bs.vibra.digital/feed.xml?site=portal&size=10",
		"https://feeds.folha.uol.com.br/emcimadahora/rss091.xml",
		"https://www.gazetadopovo.com.br/feed/rss/republica.xml",
		"https://jovempan.com.br/feed/",
		"https://www.metropoles.com/feed",
		"https://oantagonista.com.br/feed/",
		"https://rss.tecmundo.com.br/feed",
		"https://www.oficinadanet.com.br/rss/geral",
	}
	current := defaultURLSet()
	for _, feedURL := range replacements {
		if _, exists := current[feedURL]; !exists {
			t.Fatalf("verified replacement default URL missing: %s", feedURL)
		}
	}
}

func TestCustomFeedsOverrideDefaults(t *testing.T) {
	cfg, err := Load(LoadOptions{
		RequireWebhook: false,
		OS:             "linux",
		Env: map[string]string{
			"RSS_BOT_DATA": t.TempDir(),
			"RSS_FEEDS":    "https://example.com/rss",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Feeds) != 1 || cfg.Feeds[0].URL != "https://example.com/rss" {
		t.Fatalf("RSS_FEEDS should override defaults, got %#v", cfg.Feeds)
	}

	cfg, err = Load(LoadOptions{
		RequireWebhook: false,
		OS:             "linux",
		Env: map[string]string{
			"RSS_BOT_DATA":         t.TempDir(),
			"RSS_FEEDS_NEWS":       "https://example.com/news",
			"RSS_FEEDS_TECHNOLOGY": "https://example.com/tech",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Feeds) != 2 {
		t.Fatalf("category feeds should override defaults, got %#v", cfg.Feeds)
	}
}

func defaultURLSet() map[string]struct{} {
	urls := map[string]struct{}{}
	for _, group := range defaultFeedGroups() {
		for _, feedURL := range group.urls {
			urls[feedURL] = struct{}{}
		}
	}
	return urls
}
