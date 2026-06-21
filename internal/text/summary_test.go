package text

import (
	"strings"
	"testing"
)

func TestCleanSummaryRemovesBoilerplateAndDuplicates(t *testing.T) {
	title := "Título da matéria"
	description := `<p>Título da matéria</p>
<p>Primeira frase útil.</p>
<p>Primeira frase útil.</p>
<p>Clique aqui para seguir o canal no WhatsApp</p>
<p>Leia mais</p>
<p>Reprod...</p>`
	got := CleanSummary(title, description, "", 300)
	want := "Primeira frase útil."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCleanSummaryNeutralizesMentionsAndMarkdown(t *testing.T) {
	input := "## Headline\n> Quote\n[@everyone](https://evil.example) **bold** `code` ||spoiler|| <@123> <@&456> <#789> @here ```block```"
	got := CleanSummary("Different", input, "", 500)
	for _, forbidden := range []string{"@everyone", "@here", "<@123>", "<@&456>", "<#789>", "`", "||", "**", "##", ">"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("summary still contains %q: %q", forbidden, got)
		}
	}
	if !strings.Contains(got, "@\u200beveryone") || !strings.Contains(got, "@\u200bhere") {
		t.Fatalf("mentions were not neutralized readably: %q", got)
	}
}

func TestCleanSummaryTruncatesAtSentenceThenWord(t *testing.T) {
	sentence := CleanSummary("", "Primeira frase completa. Segunda frase que deve cair por limite.", "", 28)
	if sentence != "Primeira frase completa…" {
		t.Fatalf("sentence truncation got %q", sentence)
	}
	word := CleanSummary("", "Palavra longa seguida de outras palavras", "", 24)
	if word != "Palavra longa seguida…" {
		t.Fatalf("word truncation got %q", word)
	}
}

func TestCleanSummaryOmitUselessAndPreserveAccents(t *testing.T) {
	if got := CleanSummary("Título", "https://example.com/a", "", 100); got != "" {
		t.Fatalf("URL-only summary should be omitted, got %q", got)
	}
	got := CleanSummary("", "Notícias de tecnologia avançam no país.", "", 100)
	if got != "Notícias de tecnologia avançam no país." {
		t.Fatalf("accented text changed: %q", got)
	}
}

func TestCleanSummaryNoBrokenRune(t *testing.T) {
	got := CleanSummary("", strings.Repeat("😀 palavra ", 20), "", 18)
	if strings.Contains(got, "\uFFFD") || !strings.HasSuffix(got, "…") {
		t.Fatalf("bad unicode truncation: %q", got)
	}
}
