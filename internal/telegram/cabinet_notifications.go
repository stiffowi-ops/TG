package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Delays count working time inside the Monday-Friday Moscow office window.
const supplementReminderDelay = 2 * time.Hour
const supplementEscalationDelay = 4 * time.Hour

// Notify the assigned administrator 15 minutes before the one-hour deadline.
const caseClosingWarningAfter = 45 * time.Minute

func (b *Bot) caseAdminRecipients(l leadApplication) []int64 {
	if l.TakenInWork && b.adminIDs[l.TakenBy] {
		return []int64{l.TakenBy}
	}
	ids := []int64{}
	for id := range b.adminIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
func supplementAdminMarkup(l leadApplication, u leadUpdate) any {
	rows := [][]Button{{adminCaseButton(l.Number)}}
	if u.AcceptedAt == nil && !l.isResolved() {
		rows = append(rows, []Button{callbackButton("✅ Принять дополнение", fmt.Sprintf("adm:case:ack:%d:%d", l.Number, u.ID))})
	} else if u.AcceptedAt != nil {
		rows = append(rows, []Button{{"text": "✅ Дополнение принято", "callback_data": fmt.Sprintf("adm:case:view:%d", l.Number), "style": "success"}})
	}
	return markup(rows...)
}
func supplementAdminText(l leadApplication, u leadUpdate, stage string) string {
	title := "🔔 Дополнение к заявке"
	if stage == "reminder" {
		title = "⏰ Дополнение ожидает принятия"
	}
	if stage == "escalation" {
		title = "‼️ Дополнение не принято более 4 рабочих часов"
	}
	text := fmt.Sprintf("<b>%s №%04d</b>\nДополнение №%d · %s\n\n<b>Клиент:</b> %s\n<b>Телефон:</b> %s\n", title, l.Number, u.ID, caseTime(u.CreatedAt), html.EscapeString(l.Name), html.EscapeString(l.Phone))
	if l.Username != "" {
		text += "<b>Telegram:</b> @" + html.EscapeString(l.Username) + "\n"
	}
	if len(u.Locations) > 0 {
		text += "\n<b>Добавлены локации</b>\n" + locationLines(u.Locations)
	}
	if u.Comment != "" {
		text += "\n<b>Комментарий</b>\n" + html.EscapeString(u.Comment) + "\n"
	}
	text += fmt.Sprintf("\n<b>Всего локаций в заявке:</b> %d\n%s", len(l.locations()), leadStatus(l))
	return text
}

// Successful deliveries are durably acknowledged per event, stage and recipient.
// The mutex serializes the polling retry and interactive request handlers.
func (b *Bot) deliverCaseNotice(ctx context.Context, l leadApplication, key string, chatID int64, text string, keyboard any, reminderClock func() time.Time) {
	if ctx.Err() != nil || l.NoticeReceipts[key] {
		return
	}
	latest, exists := b.leads.byNumber(l.Number)
	if !exists || latest.Hidden || latest.NoticeReceipts[key] {
		return
	}
	if strings.HasPrefix(key, "update:") {
		parts := strings.Split(key, ":")
		if len(parts) != 4 {
			return
		}
		id, e := strconv.Atoi(parts[1])
		if e != nil || id <= 0 || id > len(latest.Updates) || latest.Updates[id-1].AcceptedAt != nil {
			return
		}
		if latest.isResolved() && parts[2] != "initial" {
			return
		}
		if latest.TakenInWork && b.adminIDs[latest.TakenBy] {
			owner := latest.TakenBy == chatID
			if (parts[2] == "escalation" && owner) || (parts[2] != "escalation" && !owner) {
				return
			}
		}
	}
	if strings.HasPrefix(key, "closing:") {
		deadline := latest.autoCloseAt()
		now := reminderClock()
		if latest.ServiceVersion < 2 || latest.TakenBy != chatID || !b.adminIDs[chatID] || deadline.IsZero() || now.Before(deadline.Add(-15*time.Minute)) || !now.Before(deadline) {
			return
		}
	}
	if strings.HasPrefix(key, "case:") {
		current, ok := b.leads.submitted(latest.UserID)
		if !ok || current.Number != latest.Number || latest.TakenInWork || latest.isResolved() {
			return
		}
	}
	if isAdminReminderKey(key) && !adminWorkTime(reminderClock()) {
		return
	}
	if key == "client:taken" && latest.isResolved() {
		return
	}
	raw, err := b.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML", "link_preview_options": map[string]any{"is_disabled": true}, "reply_markup": keyboard})
	if err != nil {
		log.Printf("Case %d delivery %s: %v", l.Number, key, err)
		return
	}
	var receipt struct {
		MessageID int `json:"message_id"`
	}
	_ = json.Unmarshal(raw, &receipt)
	if err := b.leads.markCaseNotice(l.Number, key, receipt.MessageID); err != nil {
		log.Printf("Case %d delivery receipt %s: %v", l.Number, key, err)
	}
}

type caseNoticeSender func(leadApplication, string, int64, string, any)

func pendingCaseReminderText(l leadApplication, stage string) string {
	title := "⏰ Заявка не взята в работу"
	delay := 2
	if stage == "escalation" {
		title = "‼️ Заявка не взята в работу"
		delay = 4
	}
	return fmt.Sprintf("<b>%s · №%04d</b>\n\nОжидает обработки более %d рабочих часов.\nРабочее время: пн–пт, 09:00–19:00 МСК.\nПолучена: %s\n\n<b>Клиент:</b> %s\n<b>Телефон:</b> %s\n\n<b>Локации</b>\n%s", title, l.Number, delay, caseTime(l.CreatedAt), html.EscapeString(l.Name), html.EscapeString(l.Phone), locationLines(l.locations()))
}
func (b *Bot) notifyPendingCaseReminder(l leadApplication, now time.Time, send caseNoticeSender) {
	if l.TakenInWork || l.isResolved() {
		return
	}
	current, ok := b.leads.submitted(l.UserID)
	if !ok || current.Number != l.Number {
		return
	}
	stage := adminReminderStage(l.CreatedAt, now)
	if stage == "" {
		return
	}
	// After downtime send only the latest due stage to each recipient, instead
	// of a 2h reminder and a 4h escalation together at the next opening.
	for _, id := range b.caseAdminRecipients(l) {
		send(l, fmt.Sprintf("case:%s:%d", stage, id), id, pendingCaseReminderText(l, stage), adminLeadMarkup(l))
	}
}
// A durable one-time reminder to the current owner; the existing delivery
// receipts prevent duplicates across retries and bot restarts.
func (b *Bot) notifyClosingCaseReminder(l leadApplication, now time.Time, send caseNoticeSender) {
	deadline := l.autoCloseAt()
	if l.ServiceVersion < 2 || deadline.IsZero() || !b.adminIDs[l.TakenBy] || now.Before(l.TakenAt.Add(caseClosingWarningAfter)) || !now.Before(deadline) {
		return
	}
	unaccepted := 0
	for _, u := range l.Updates {
		if u.AcceptedAt == nil {
			unaccepted++
		}
	}
	message := fmt.Sprintf("⏰ <b>Заявка №%04d скоро будет автоматически завершена.</b>\n\nДо завершения осталось не более 15 минут.\nВремя завершения: <b>%s</b>.\nНепринятых дополнений: %d.", l.Number, caseTime(deadline), unaccepted)
	send(l, fmt.Sprintf("closing:45:%d", l.TakenBy), l.TakenBy, message, markup([]Button{adminCaseButton(l.Number)}))
}

func (b *Bot) notifySupplementReminder(l leadApplication, u leadUpdate, now time.Time, send caseNoticeSender) {
	stage := adminReminderStage(u.CreatedAt, now)
	if stage == "" {
		return
	}
	if stage == "reminder" {
		for _, id := range b.caseAdminRecipients(l) {
			send(l, updateKey(u.ID, "reminder", id), id, supplementAdminText(l, u, "reminder"), supplementAdminMarkup(l, u))
		}
		return
	}
	// An assigned owner still gets their one outstanding reminder after a
	// restart, while other administrators receive the escalation, once each.
	owner := int64(0)
	if l.TakenInWork && b.adminIDs[l.TakenBy] {
		owner = l.TakenBy
	}
	if owner > 0 {
		send(l, updateKey(u.ID, "reminder", owner), owner, supplementAdminText(l, u, "reminder"), supplementAdminMarkup(l, u))
	}
	ids := []int64{}
	for id := range b.adminIDs {
		if id != owner {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		send(l, updateKey(u.ID, "escalation", id), id, supplementAdminText(l, u, "escalation"), supplementAdminMarkup(l, u))
	}
}
func (b *Bot) notifyCaseEvents(ctx context.Context, now time.Time) {
	if b.leads == nil {
		return
	}
	b.notifySelectedCaseEvents(ctx, now, b.leads.visibleSubmitted())
}

func (b *Bot) notifySelectedCaseEvents(ctx context.Context, now time.Time, cases []leadApplication) {
	startedAt := time.Now()
	b.leadNotifyMu.Lock()
	defer b.leadNotifyMu.Unlock()
	// Recheck the office window before every actual send. Account for the time
	// spent on preceding API requests so a batch cannot run past 19:00.
	reminderClock := func() time.Time { return now.Add(time.Since(startedAt)) }
	send := caseNoticeSender(func(l leadApplication, key string, id int64, text string, keyboard any) {
		b.deliverCaseNotice(ctx, l, key, id, text, keyboard, reminderClock)
	})
	for _, l := range cases {
		b.notifyPendingCaseReminder(l, reminderClock(), send)
		if l.ServiceVersion == 0 {
			continue
		}
		if l.TakenInWork && !l.isResolved() {
			b.notifyClosingCaseReminder(l, reminderClock(), send)
			note := "Статус доступен в разделе «Мои заявки»."
			if !l.supplementLimitReached() {
				note = "Статус и возможность дополнить обращение доступны в разделе «Мои заявки»."
			}
			send(l, "client:taken", l.UserID, fmt.Sprintf("🛠 <b>Ваше обращение №%04d взято в работу.</b>\n\n%s", l.Number, note), markup([]Button{caseButton("📂 Открыть обращение", l.Number)}))
		}
		for _, u := range l.Updates {
			if u.AcceptedAt != nil {
				send(l, fmt.Sprintf("client:ack:%d", u.ID), l.UserID, fmt.Sprintf("✅ <b>Специалист принял дополнение №%d к обращению №%04d.</b>", u.ID, l.Number), markup([]Button{caseButton("📂 Открыть обращение", l.Number)}))
				continue
			}
			for _, id := range b.caseAdminRecipients(l) {
				send(l, updateKey(u.ID, "initial", id), id, supplementAdminText(l, u, "initial"), supplementAdminMarkup(l, u))
			}
			if !l.isResolved() {
				b.notifySupplementReminder(l, u, reminderClock(), send)
			}
		}
		if l.isResolved() {
			text := processedText
			if l.Resolution != "" {
				text += "\n\n<b>Итог</b>\n" + html.EscapeString(l.Resolution)
			}
			send(l, "client:resolved", l.UserID, text, markup([]Button{caseButton(fmt.Sprintf("📂 Заявка №%04d", l.Number), l.Number), cabinetButton()}))
		}
	}
}

func (b *Bot) refreshSupplementMessages(ctx context.Context, l leadApplication) {
	for key, mid := range l.NoticeMessages {
		if !strings.HasPrefix(key, "update:") || mid <= 0 {
			continue
		}
		p := strings.Split(key, ":")
		if len(p) != 4 {
			continue
		}
		id, e := strconv.Atoi(p[1])
		admin, e2 := strconv.ParseInt(p[3], 10, 64)
		if e != nil || e2 != nil || !b.adminIDs[admin] || id <= 0 || id > len(l.Updates) {
			continue
		}
		u := l.Updates[id-1]
		if u.AcceptedAt == nil && !l.isResolved() {
			continue
		}
		if _, err := b.call(ctx, "editMessageReplyMarkup", map[string]any{"chat_id": admin, "message_id": mid, "reply_markup": supplementAdminMarkup(l, u)}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
			log.Printf("Case supplement message refresh: %v", err)
		}
	}
}
