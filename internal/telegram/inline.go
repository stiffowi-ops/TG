package telegram

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"locationbot/internal/analytics"
	"locationbot/internal/catalog"
)

// Inline search uses Telegram's native suggestions, updated as the user types.
type inlineQuery struct {
	ID     string `json:"id"`
	From   user   `json:"from"`
	Query  string `json:"query"`
	Offset string `json:"offset"`
}

const inlineLocationLimit = 15

func inlineSearchButton() Button {
	return Button{"text": "🔎 Поиск локации", "switch_inline_query_current_chat": ""}
}

type inlineMatch struct {
	location catalog.Location
	rank     int
}

// Search only approved location titles, not free-text descriptions. Prefer
// prefix matches, then word starts, then fragments. Ignore case, ё/е and Метро.
func inlineLocationMatches(all []catalog.Location, query string, limit int) []catalog.Location {
	if limit <= 0 {
		return nil
	}
	query = catalog.Normalize(query)
	words := strings.Fields(query)
	matches := make([]inlineMatch, 0, len(all))
	for _, l := range all {
		if l.ID == "" || l.Title == "" {
			continue
		}
		name := catalog.Normalize(l.Title)
		rank := 0
		if query != "" {
			rank = 3
			switch {
			case strings.HasPrefix(name, query):
				rank = 0
			case inlineAnyWordStarts(name, query):
				rank = 1
			case strings.Contains(name, query):
				rank = 2
			default:
				allWords := true
				for _, word := range words {
					if !strings.Contains(name, word) {
						allWords = false
						break
					}
				}
				if !allWords {
					continue
				}
			}
		}
		matches = append(matches, inlineMatch{location: l, rank: rank})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		ka, kb := stationSortKey(a.location.Title), stationSortKey(b.location.Title)
		if ka != kb {
			return ka < kb
		}
		return a.location.ID < b.location.ID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	results := make([]catalog.Location, 0, len(matches))
	for _, m := range matches {
		results = append(results, m.location)
	}
	return results
}

func inlineAnyWordStarts(name, prefix string) bool {
	for _, word := range strings.Fields(name) {
		if strings.HasPrefix(word, prefix) {
			return true
		}
	}
	return false
}

// Inline result navigation is private, because favorites are personal.
func inlineLocationRows(id string, saved bool) [][]Button {
	label := "⭐ Добавить в избранное"
	if saved {
		label = "✅ Убрать из избранного"
	}
	return [][]Button{
		{callbackButton(label, "fav:inline:"+id)},
		{callbackButton("📝 Оставить заявку", "inline:nav:lead:"+id)},
		{callbackButton("⭐ Перейти в избранное", "inline:nav:favorites"), callbackButton("🔎 Поиск локации", "search:catalog")},
		{callbackButton("🏠 Главное меню", "inline:nav:menu")},
	}
}

func (b *Bot) answerLocationInlineQuery(ctx context.Context, query *inlineQuery) error {
	if query == nil || query.ID == "" {
		return nil
	}
	results := make([]map[string]any, 0, inlineLocationLimit)
	if b.store != nil {
		for _, raw := range inlineLocationMatches(b.store.List(), query.Query, inlineLocationLimit) {
			loc := publicLocation(raw)
			if !catalog.ValidID(loc.ID) || len("fav:inline:"+loc.ID) > 64 {
				continue
			}
			description := strings.TrimSpace(loc.Landmarks)
			if description == "" {
				description = strings.TrimSpace(loc.LineStatus)
			}
			if description == "" {
				description = "Открыть карточку локации"
			}
			saved := b.favorites != nil && b.favorites.Contains(query.From.ID, loc.ID)
			results = append(results, map[string]any{
				"type": "article", "id": "karty:" + loc.ID, "title": loc.Title,
				"description": limitRunes(description, 125),
				"input_message_content": map[string]any{
					"rich_message": map[string]any{"html": richLocationHTML(loc, b.mapURL)},
				},
				"reply_markup": markup(inlineLocationRows(loc.ID, saved)...),
			})
		}
	}
	_, err := b.call(ctx, "answerInlineQuery", map[string]any{
		"inline_query_id": query.ID, "results": results, "cache_time": 0,
		"is_personal": true, "next_offset": "",
	})
	return err
}

func (b *Bot) replyInlineCallback(ctx context.Context, callback *callbackQuery, text string) error {
	_, err := b.call(ctx, "answerCallbackQuery", map[string]any{
		"callback_query_id": callback.ID, "text": text, "show_alert": false,
	})
	return err
}

// The inline result belongs to the user's message, not a bot-owned chat
// message. Edit its inline markup when Telegram supplies inline_message_id.
// Shared group cards do not expose personal favorites state to others.
func (b *Bot) toggleInlineFavorite(ctx context.Context, callback *callbackQuery) error {
	if callback == nil {
		return nil
	}
	id := strings.TrimPrefix(callback.Data, "fav:inline:")
	if b.store == nil || b.favorites == nil || !catalog.ValidID(id) || callback.From.ID <= 0 {
		return b.replyInlineCallback(ctx, callback, "Не удалось обновить избранное.")
	}
	if _, ok := b.store.Find(id); !ok {
		return b.replyInlineCallback(ctx, callback, fmt.Sprintf("Локация %s больше не найдена", id))
	}
	saved, err := b.favorites.Toggle(callback.From.ID, id)
	if err != nil {
		return b.replyInlineCallback(ctx, callback, "Не удалось обновить избранное.")
	}
	kind := analytics.FavoriteRemoved
	if saved {
		kind = analytics.FavoriteAdded
	}
	b.recordLocationAnalytics(callback.From.ID, id, kind)
	// Telegram may supply inline_message_id without callback.Message even in
	// private chats. Updating a shared card changes only its button label; the
	// personal favorites list is never sent into that chat.
	rows := markup(inlineLocationRows(id, saved)...)
	payload := map[string]any{"reply_markup": rows}
	if callback.InlineMessageID != "" {
		payload["inline_message_id"] = callback.InlineMessageID
	} else if callback.Message != nil && callback.Message.MessageID > 0 {
		payload["chat_id"] = callback.Message.Chat.ID
		payload["message_id"] = callback.Message.MessageID
	}
	if len(payload) > 1 {
		if _, editErr := b.call(ctx, "editMessageReplyMarkup", payload); editErr != nil {
			log.Printf("Inline favorite keyboard update failed: %v", editErr)
		}
	}
	if saved {
		return b.replyInlineCallback(ctx, callback, "⭐ Добавлено в избранное")
	}
	return b.replyInlineCallback(ctx, callback, "✅ Убрано из избранного")
}

// Normal and inline cards use the same screen rendering functions.
// The main menu may safely replace an inline result, because it contains no
// personal data. Favorites always open in the caller's private chat, never
// inside a possibly public inline result.
func (b *Bot) inlineNavigate(ctx context.Context, callback *callbackQuery) error {
	if callback == nil {
		return nil
	}
	if err := b.pauseClientForms(callback.From.ID); err != nil {
		return err
	}
	if strings.HasPrefix(callback.Data, "inline:nav:lead:") {
		return b.openInlineLead(ctx, callback)
	}
	if callback.From.ID <= 0 {
		return b.replyInlineCallback(ctx, callback, "Не удалось открыть раздел.")
	}
	if err := b.replyInlineCallback(ctx, callback, ""); err != nil {
		log.Printf("Acknowledge inline navigation: %v", err)
	}
	chatID := callback.From.ID
	switch callback.Data {
	case "inline:nav:favorites":
		return b.showFavorites(b.privateScreenContext(ctx, chatID), chatID, chatID)
	case "inline:nav:menu":
		if callback.InlineMessageID != "" {
			return b.welcome(withInlineNavigation(ctx, callback.InlineMessageID), chatID, callback.From)
		}
		if callback.Message != nil && callback.Message.Chat.Type == "private" {
			return b.welcome(withNavigation(ctx, callback.Message.Chat.ID, callback.Message.MessageID), callback.Message.Chat.ID, callback.From)
		}
		return b.welcome(b.privateScreenContext(ctx, chatID), chatID, callback.From)
	default:
		return nil
	}
}
