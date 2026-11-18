package telegram

import (
	"context"
	"encoding/json"
	"log"
	"strings"
)

// A callback belongs to a specific bot message, so menus can replace that
// message instead of adding another bubble to the private chat.
// We do not delete user messages, including messages sent via Inline Mode.
type navigationKey struct{}

// Inline results have inline_message_id instead of a normal Message. Keeping
// that identifier in the same request context lets our normal screen
// renderers update the inline card without opening another chat message.
type inlineNavigationKey struct{}

func withInlineNavigation(ctx context.Context, inlineID string) context.Context {
	if inlineID == "" {
		return ctx
	}
	return context.WithValue(ctx, inlineNavigationKey{}, inlineID)
}

func inlineNavigationFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(inlineNavigationKey{}).(string)
	return id, ok && id != ""
}

func (b *Bot) editInlineNavigationMessage(ctx context.Context, inlineID, value string, keyboard any) error {
	_, err := b.call(ctx, "editMessageText", map[string]any{
		"inline_message_id": inlineID,
		"text": value,
		"parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
		"reply_markup": keyboard,
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
		return nil
	}
	return err
}

func (b *Bot) rememberBotScreen(chatID int64, raw []byte) {
	var result struct {
		MessageID int `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &result); err == nil && result.MessageID > 0 {
		b.rememberBotScreenID(chatID, result.MessageID)
	}
}

func (b *Bot) rememberBotScreenID(chatID int64, messageID int) {
	if chatID <= 0 || messageID <= 0 { return }
	b.screenMu.Lock()
	defer b.screenMu.Unlock()
	if b.lastScreens == nil { b.lastScreens = map[int64]int{} }
	b.lastScreens[chatID] = messageID
	delete(b.searchPrompts, chatID)
}

func (b *Bot) privateScreenContext(ctx context.Context, chatID int64) context.Context {
	b.screenMu.Lock()
	messageID := b.lastScreens[chatID]
	b.screenMu.Unlock()
	if messageID > 0 { return withNavigation(ctx, chatID, messageID) }
	return ctx
}

func (b *Bot) sendNewText(ctx context.Context, chatID int64, value string, keyboard any) error {
	result, err := b.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text": value,
		"parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
		"reply_markup": keyboard,
	})
	if err == nil { b.rememberBotScreen(chatID, result) }
	return err
}


// When a user selects one of our inline search suggestions, Telegram creates
// a user-owned message. We cannot delete that message, but we can remove the
// obsolete bot-owned search prompt that preceded it.
func (b *Bot) markSearchPrompt(chatID int64) {
	b.screenMu.Lock()
	defer b.screenMu.Unlock()
	messageID := b.lastScreens[chatID]
	if messageID <= 0 { return }
	if b.searchPrompts == nil { b.searchPrompts = make(map[int64]int) }
	b.searchPrompts[chatID] = messageID
}

func (b *Bot) clearSelectedSearchPrompt(ctx context.Context, chatID int64) {
	b.screenMu.Lock()
	messageID := b.searchPrompts[chatID]
	delete(b.searchPrompts, chatID)
	if messageID > 0 && b.lastScreens[chatID] == messageID {
		delete(b.lastScreens, chatID)
	}
	b.screenMu.Unlock()
	if messageID > 0 {
		_, err := b.call(ctx, "deleteMessage", map[string]any{
			"chat_id": chatID, "message_id": messageID,
		})
		if err != nil { log.Printf("Delete obsolete bot search prompt: %v", err) }
	}
}

func supportsPublicInlineNavigation(data string) bool {
	switch {
	case data == "menu", data == "list", data == "near", data == "search":
		return true
	case strings.HasPrefix(data, "search:"), strings.HasPrefix(data, "page:"),
		strings.HasPrefix(data, "loc:"), strings.HasPrefix(data, "fav:toggle:"):
		return true
	default:
		return false
	}
}

type navigationTarget struct {
	chatID    int64
	messageID int
}

func navigationFromContext(ctx context.Context, chatID int64) (int, bool) {
	target, ok := ctx.Value(navigationKey{}).(navigationTarget)
	if !ok || target.messageID <= 0 || target.chatID != chatID {
		return 0, false
	}
	return target.messageID, true
}

func withNavigation(ctx context.Context, chatID int64, messageID int) context.Context {
	if messageID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, navigationKey{}, navigationTarget{
		chatID: chatID, messageID: messageID,
	})
}

func (b *Bot) editNavigationMessage(ctx context.Context, chatID int64, messageID int, text string, keyboard any) error {
	_, err := b.call(ctx, "editMessageText", map[string]any{
		"chat_id": chatID,
		"message_id": messageID,
		"text": text,
		"parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
		"reply_markup": keyboard,
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
		return nil
	}
	return err
}

// Telegram cannot edit old, inaccessible or non-bot messages. In those cases
// a new message is preferable to a broken button.
func (b *Bot) sendOrEdit(ctx context.Context, chatID int64, text string, keyboard any) error {
	if messageID, ok := navigationFromContext(ctx, chatID); ok {
		if err := b.editNavigationMessage(ctx, chatID, messageID, text, keyboard); err == nil {
			return nil
		} else {
			log.Printf("Inline menu edit failed for message %d: %v; sending new screen", messageID, err)
		}
	}
	_, err := b.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text": text,
		"parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
		"reply_markup": keyboard,
	})
	return err
}
