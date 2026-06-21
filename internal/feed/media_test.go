package feed

import (
	"testing"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

func TestArticleImagePriority(t *testing.T) {
	item := &gofeed.Item{
		Image: &gofeed.Image{URL: "https://example.com/item.jpg"},
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://example.com/enclosure.jpg", Type: "image/jpeg"},
		},
		Extensions: ext.Extensions{
			"media": {"content": []ext.Extension{{Attrs: map[string]string{"url": "https://example.com/media.jpg", "type": "image/jpeg"}}}},
		},
		Description: `<img src="https://example.com/description.jpg">`,
		Content:     `<img src="https://example.com/content.jpg">`,
	}
	if got := articleImageURL(item, "https://example.com/article"); got != "https://example.com/item.jpg" {
		t.Fatalf("got %q", got)
	}

	item.Image = nil
	if got := articleImageURL(item, "https://example.com/article"); got != "https://example.com/enclosure.jpg" {
		t.Fatalf("got %q", got)
	}

	item.Enclosures = nil
	if got := articleImageURL(item, "https://example.com/article"); got != "https://example.com/media.jpg" {
		t.Fatalf("got %q", got)
	}

	item.Extensions = nil
	if got := articleImageURL(item, "https://example.com/article"); got != "https://example.com/description.jpg" {
		t.Fatalf("got %q", got)
	}

	item.Description = ""
	if got := articleImageURL(item, "https://example.com/article"); got != "https://example.com/content.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestArticleImageMediaExtensions(t *testing.T) {
	tests := []struct {
		name      string
		extension ext.Extensions
		want      string
	}{
		{
			name: "media content",
			extension: ext.Extensions{
				"media": {"content": []ext.Extension{{Attrs: map[string]string{"url": "https://cdn.example/content.png", "medium": "image"}}}},
			},
			want: "https://cdn.example/content.png",
		},
		{
			name: "media thumbnail",
			extension: ext.Extensions{
				"media": {"thumbnail": []ext.Extension{{Attrs: map[string]string{"url": "https://cdn.example/thumb.png"}}}},
			},
			want: "https://cdn.example/thumb.png",
		},
		{
			name: "media image",
			extension: ext.Extensions{
				"media": {"image": []ext.Extension{{Value: "https://cdn.example/image.webp"}}},
			},
			want: "https://cdn.example/image.webp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := articleImageURL(&gofeed.Item{Extensions: tt.extension}, "https://example.com/article")
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestArticleImageFilteringAndResolution(t *testing.T) {
	tests := []struct {
		name string
		item *gofeed.Item
		want string
	}{
		{name: "relative image", item: &gofeed.Item{Description: `<img src="/images/a.jpg">`}, want: "https://example.com/images/a.jpg"},
		{name: "audio enclosure ignored", item: &gofeed.Item{Enclosures: []*gofeed.Enclosure{{URL: "https://example.com/audio.mp3", Type: "audio/mpeg"}}}, want: ""},
		{name: "video enclosure ignored", item: &gofeed.Item{Enclosures: []*gofeed.Enclosure{{URL: "https://example.com/video.mp4", Type: "video/mp4"}}}, want: ""},
		{name: "data URL ignored", item: &gofeed.Item{Description: `<img src="data:image/png;base64,AAAA">`}, want: ""},
		{name: "javascript URL ignored", item: &gofeed.Item{Description: `<img src="javascript:alert(1)">`}, want: ""},
		{name: "malformed URL ignored", item: &gofeed.Item{Image: &gofeed.Image{URL: "http://[bad"}}, want: ""},
		{name: "tracking pixel ignored", item: &gofeed.Item{Description: `<img width="1" height="1" src="https://example.com/pixel.jpg"><img src="https://example.com/real.jpg">`}, want: "https://example.com/real.jpg"},
		{name: "no image", item: &gofeed.Item{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := articleImageURL(tt.item, "https://example.com/article")
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestAuthorSourceAndIconExtraction(t *testing.T) {
	feed := &gofeed.Feed{
		Link:  "https://publisher.example/news",
		Image: &gofeed.Image{URL: "/logo.png"},
	}
	item := &gofeed.Item{
		Authors: []*gofeed.Person{{Name: ""}, {Name: "Reporter"}},
		Link:    "https://articles.example/post",
	}
	if got := articleAuthor(item); got != "Reporter" {
		t.Fatalf("author got %q", got)
	}
	if got := sourceURL(feed, item.Link, "https://feed.example/rss"); got != "https://publisher.example/news" {
		t.Fatalf("source URL got %q", got)
	}
	if got := sourceIconURL(feed, "https://publisher.example/news"); got != "https://publisher.example/logo.png" {
		t.Fatalf("source icon got %q", got)
	}
}
