package feed

import (
	"sort"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
)

func OldestFirst(articles []model.Article) []model.Article {
	ordered := append([]model.Article(nil), articles...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		if left.PublishedAt != nil && right.PublishedAt != nil && !left.PublishedAt.Equal(*right.PublishedAt) {
			return left.PublishedAt.Before(*right.PublishedAt)
		}
		if left.PublishedAt != nil && right.PublishedAt == nil {
			return true
		}
		if left.PublishedAt == nil && right.PublishedAt != nil {
			return false
		}
		return left.Sequence > right.Sequence
	})
	return ordered
}

func NewestFirst(articles []model.Article) []model.Article {
	ordered := OldestFirst(articles)
	for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
		ordered[i], ordered[j] = ordered[j], ordered[i]
	}
	return ordered
}
