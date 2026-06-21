package model

import "time"

type FeedConfig struct {
	URL      string
	Source   string
	Category string
	Emoji    string
}

type FeedState struct {
	URL                 string
	Source              string
	Category            string
	Initialized         bool
	ETag                string
	LastModified        string
	LastCheckedAt       *time.Time
	LastSuccessAt       *time.Time
	ConsecutiveFailures int
	NextAttemptAt       *time.Time
}

type Article struct {
	FeedURL        string
	Source         string
	Category       string
	CategoryEmoji  string
	GUID           string
	Link           string
	NormalizedLink string
	Title          string
	Description    string
	Content        string
	ImageURL       string
	AuthorName     string
	SourceURL      string
	SourceIconURL  string
	PublishedAt    *time.Time
	ArticleKey     string
	LegacyHash     string
	Sequence       int
}

type CycleStats struct {
	FeedsAttempted        int
	FeedsSuccessful       int
	FeedsFailed           int
	PostsSent             int
	DiscordFailures       int
	ConsecutiveTotalFails int
}
