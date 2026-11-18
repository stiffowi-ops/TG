package telegram

import (
	"os"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"locationbot/internal/syncer"
)

const botVersion = "2.18.2"

func (b *Bot) configureTelegramMenu(ctx context.Context) {
	_, err := b.call(ctx, "setMyCommands", map[string]any{"commands": []map[string]string{
		{"command": "start", "description": "Главное меню"},
	}})
	if err != nil && ctx.Err() == nil {
		log.Printf("Configure Telegram commands: %v", err)
	}
	menuButton:=any(map[string]string{"type":"commands"})
	if appURL:=strings.TrimSpace(os.Getenv("MINIAPP_URL"));strings.HasPrefix(appURL,"https://") {
		menuButton=map[string]any{"type":"web_app","text":"Открыть приложение","web_app":map[string]string{"url":appURL}}
	}
	_, err = b.call(ctx, "setChatMenuButton", map[string]any{"menu_button":menuButton})
	if err != nil && ctx.Err() == nil {
		log.Printf("Configure Telegram menu: %v", err)
	}
}

func (b *Bot) runKeyboardCleanup(ctx context.Context) {
	cleaned := make(map[int64]bool)
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case chatID := <-b.keyboardCleanup:
			if cleaned[chatID] {
				continue
			}
			raw, err := b.call(ctx, "sendMessage", map[string]any{
				"chat_id": chatID, "text": "·", "disable_notification": true,
				"reply_markup": map[string]any{"remove_keyboard": true},
			})
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("Remove legacy keyboard: %v", err)
				}
				continue
			}
			cleaned[chatID] = true
			var result struct {
				MessageID int `json:"message_id"`
			}
			if json.Unmarshal(raw, &result) == nil && result.MessageID > 0 {
				_, err = b.call(ctx, "deleteMessage", map[string]any{"chat_id": chatID, "message_id": result.MessageID})
				if err != nil && ctx.Err() == nil {
					log.Printf("Delete keyboard reset: %v", err)
				}
			}
		}
	}
}

// A buffered wakeup coalesces bursts. Leads, pending notices and delivery
// receipts live in the store, so a restart cannot discard a pending delivery.
func (b *Bot) queueLeadDelivery(refreshNumber int64) {
	if refreshNumber > 0 {
		b.deliveryMu.Lock()
		if b.refreshCases == nil {
			b.refreshCases = make(map[int64]bool)
		}
		b.refreshCases[refreshNumber] = true
		b.deliveryMu.Unlock()
	}
	select {
	case b.deliveryWake <- struct{}{}:
	default:
	}
}

func (b *Bot) processLeadDeliveries(ctx context.Context) {
	b.deliveryMu.Lock()
	refresh := b.refreshCases
	b.refreshCases = nil
	b.deliveryMu.Unlock()
	for number := range refresh {
		if ctx.Err() != nil {
			return
		}
		if l, ok := b.leads.byNumber(number); ok && !l.Hidden {
			b.refreshLeadAdminMessages(ctx, l)
			b.refreshSupplementMessages(ctx, l)
			b.notifyLeadAdmins(ctx, l)
			b.notifySelectedCaseEvents(ctx, time.Now().UTC(), []leadApplication{l})
		}
	}
}

func (b *Bot) runLeadDelivery(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	b.retryPendingLeadNotifications(ctx)
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.retryPendingLeadNotifications(ctx)
		case <-b.deliveryWake:
			b.processLeadDeliveries(ctx)
		}
	}
}

func (b *Bot) startManualSync(ctx context.Context, chatID int64) error {
	back := markup([]Button{callbackButton("⬅️ Система", "adm:tools")})
	if b.syncer == nil {
		return b.Send(ctx, chatID, "Синхронизация недоступна.", back)
	}
	if !b.manualSync.TryLock() {
		return b.Send(ctx, chatID, "Синхронизация уже выполняется.", back)
	}
	if err := b.Send(ctx, chatID, "☁️ Обновляю данные Яндекс Диска…", back); err != nil {
		b.manualSync.Unlock()
		return err
	}
	go func() {
		defer b.manualSync.Unlock()
		syncCtx, cancel := context.WithTimeout(ctx, 7*time.Minute)
		defer cancel()
		info, err := b.syncer.TryRun(syncCtx)
		if ctx.Err() != nil {
			return
		}
		var text string
		switch {
		case errors.Is(err, syncer.ErrBusy):
			text = "Синхронизация уже выполняется."
		case err != nil:
			text = "Синхронизация не удалась. " + html.EscapeString(err.Error())
		default:
			text = fmt.Sprintf("✅ Файлов: %d · скачано: %d · без изменений: %d · ошибок: %d", info.Total, info.Downloaded, info.Skipped, len(info.Failures))
			if len(info.Failures) > 0 {
				text += "\n" + html.EscapeString(strings.Join(info.Failures[:min(3, len(info.Failures))], "\n"))
			}
		}
		// A background completion must not replace the screen the admin has
		// opened since starting the sync, or change the tracked search prompt.
		if _, err := b.call(ctx, "sendMessage", map[string]any{
			"chat_id": chatID, "text": text, "parse_mode": "HTML", "reply_markup": back,
		}); err != nil && ctx.Err() == nil {
			log.Printf("Report manual catalog sync: %v", err)
		}
	}()
	return nil
}
