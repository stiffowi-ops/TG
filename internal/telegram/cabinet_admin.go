package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
)

func adminCaseButton(n int64) Button {
	return callbackButton("📂 Открыть заявку", fmt.Sprintf("adm:case:view:%d", n))
}
func (b *Bot) showAdminCase(ctx context.Context, chatID, number int64, page int) error {
	l, ok := b.leads.byNumber(number)
	if !ok || l.Hidden {
		return b.Send(ctx, chatID, "Обращение недоступно.", markup([]Button{callbackButton("⬅️ К заявкам", "adm:leads")}))
	}
	text := adminLeadText(l)
	rows := adminLeadMarkup(l)["inline_keyboard"].([][]Button)
	// The detail view is the destination, so its Open button is redundant.
	if len(rows) > 0 {
		rows = rows[1:]
	}
	pending := []leadUpdate{}
	for _, u := range l.Updates {
		if !l.isResolved() && u.AcceptedAt == nil {
			pending = append(pending, u)
		}
	}
	pages := max(1, (len(pending)+4)/5)
	page = max(0, min(page, pages-1))
	for _, u := range pending[page*5 : min((page+1)*5, len(pending))] {
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("✅ Принять дополнение №%d", u.ID), fmt.Sprintf("adm:case:ack:%d:%d", number, u.ID))})
	}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️ Дополнения", fmt.Sprintf("adm:case:view:%d:%d", number, page-1)))
	}
	if page+1 < pages {
		nav = append(nav, callbackButton("Дополнения ▶️", fmt.Sprintf("adm:case:view:%d:%d", number, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{callbackButton("🕓 Полная история", fmt.Sprintf("adm:case:timeline:%d:0", number))}, []Button{callbackButton("⬅️ К заявкам", "adm:leads")})
	return b.Send(ctx, chatID, text, markup(rows...))
}
func (b *Bot) handleCaseAdminCallback(ctx context.Context, from user, data string) error {
	if !b.isAdmin(from.ID) || b.leads == nil {
		return nil
	}
	p := strings.Split(data, ":")
	if len(p) < 4 {
		return nil
	}
	if p[2] == "list" {
		page, err := strconv.Atoi(p[3])
		if err != nil {
			return nil
		}
		return b.showAdminLeadsPage(ctx, from.ID, page)
	}
	number, err := strconv.ParseInt(p[3], 10, 64)
	if err != nil || number <= 0 {
		return nil
	}
	l, ok := b.leads.byNumber(number)
	if !ok || l.Hidden {
		return b.showAdminLeads(ctx, from.ID)
	}
	switch p[2] {
	case "view":
		page := 0
		if len(p) > 4 {
			page, _ = strconv.Atoi(p[4])
		}
		return b.showAdminCase(ctx, from.ID, number, page)
	case "timeline":
		page := 0
		if len(p) > 4 {
			page, _ = strconv.Atoi(p[4])
		}
		return b.showCaseTimeline(ctx, from.ID, l, page, true)
	case "ack":
		if len(p) != 5 {
			return nil
		}
		id, err := strconv.Atoi(p[4])
		if err != nil {
			return nil
		}
		updated, _, err := b.leads.acceptUpdate(number, id, from.ID)
		if err != nil {
			return b.Send(ctx, from.ID, html.EscapeString(err.Error()), markup([]Button{adminCaseButton(number)}))
		}
		b.queueLeadDelivery(updated.Number)
		return b.showAdminCase(ctx, from.ID, number, 0)
	case "resolve":
		updated, changed, err := b.leads.resolveCase(number, from.ID)
		if err != nil {
			return b.Send(ctx, from.ID, html.EscapeString(err.Error()), markup([]Button{adminCaseButton(number)}))
		}
		if changed {
			b.queueLeadDelivery(updated.Number)
		}
		return b.showAdminCase(ctx, from.ID, number, 0)
	case "finish": // Old confirmation messages reopen the current card.
		return b.showAdminCase(ctx, from.ID, number, 0)
	}
	return nil
}
