package discord

import (
	"errors"
	"strings"
)

const (
	EmbedTitleLimit       = 256
	EmbedDescriptionLimit = 4096
	EmbedAuthorNameLimit  = 256
	EmbedFooterTextLimit  = 2048
	EmbedTextLimit        = 6000
	MaxEmbedsPerMessage   = 10
)

type Message struct {
	Content         string          `json:"content,omitempty"`
	Embeds          []Embed         `json:"embeds,omitempty"`
	AllowedMentions AllowedMentions `json:"allowed_mentions"`
}

type AllowedMentions struct {
	Parse []string `json:"parse"`
}

type Embed struct {
	Author      *EmbedAuthor `json:"author,omitempty"`
	Title       string       `json:"title,omitempty"`
	URL         string       `json:"url,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Image       *EmbedImage  `json:"image,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

type EmbedAuthor struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type EmbedImage struct {
	URL string `json:"url,omitempty"`
}

type EmbedFooter struct {
	Text string `json:"text,omitempty"`
}

func NewMessage(embeds ...Embed) Message {
	return Message{
		Embeds: embeds,
		AllowedMentions: AllowedMentions{
			Parse: []string{},
		},
	}
}

func (m Message) Validate() error {
	if m.AllowedMentions.Parse == nil {
		return errors.New("allowed_mentions.parse must be an explicit empty array")
	}
	if len(m.AllowedMentions.Parse) != 0 {
		return errors.New("allowed_mentions.parse must be empty")
	}
	if strings.TrimSpace(m.Content) == "" && len(m.Embeds) == 0 {
		return errors.New("discord message has no content or embeds")
	}
	if len(m.Embeds) > MaxEmbedsPerMessage {
		return errors.New("discord message has too many embeds")
	}
	total := 0
	for _, embed := range m.Embeds {
		if strings.TrimSpace(embed.Title) == "" && strings.TrimSpace(embed.Description) == "" {
			return errors.New("discord embed must contain a title or description")
		}
		if len([]rune(embed.Title)) > EmbedTitleLimit {
			return errors.New("discord embed title exceeds limit")
		}
		if len([]rune(embed.Description)) > EmbedDescriptionLimit {
			return errors.New("discord embed description exceeds limit")
		}
		total += len([]rune(embed.Title)) + len([]rune(embed.Description))
		if embed.Author != nil {
			if len([]rune(embed.Author.Name)) > EmbedAuthorNameLimit {
				return errors.New("discord embed author name exceeds limit")
			}
			total += len([]rune(embed.Author.Name))
		}
		if embed.Footer != nil {
			if len([]rune(embed.Footer.Text)) > EmbedFooterTextLimit {
				return errors.New("discord embed footer text exceeds limit")
			}
			total += len([]rune(embed.Footer.Text))
		}
	}
	if total > EmbedTextLimit {
		return errors.New("discord embed text exceeds combined limit")
	}
	return nil
}
