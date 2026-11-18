package telegram

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"
)

const visitorFavoritePreview = 12
const visitorFavoritePageSize = 10

type visitorFavorite struct{ id, name string }

func (b *Bot) visitorFavorites(userID int64) []visitorFavorite {
	items := []visitorFavorite{}
	for _, id := range b.favorites.IDs(userID) {
		name := "Локация отсутствует в текущем каталоге"
		if b.store != nil {
			if l, ok := b.store.Find(id); ok {
				name = publicText(l.Title)
			}
		}
		items = append(items, visitorFavorite{id: id, name: name})
	}
	sort.SliceStable(items, func(i, j int) bool { return stationSortKey(items[i].name) < stationSortKey(items[j].name) })
	return items
}
func visitorFavoriteLines(items []visitorFavorite) string {
	var out strings.Builder
	for _, item := range items {
		out.WriteString("• " + html.EscapeString(limitCaptionUnits(item.name, 70)) + " (<code>" + html.EscapeString(item.id) + "</code>)\n")
	}
	return out.String()
}
func visitorFavoritesPreview(items []visitorFavorite) string {
	text := fmt.Sprintf("⭐ <b>Избранное · %d</b>\n", len(items))
	if len(items) == 0 {
		return text + "Ничего не добавлено в избранное."
	}
	text += visitorFavoriteLines(items[:min(visitorFavoritePreview, len(items))])
	if len(items) > visitorFavoritePreview {
		text += fmt.Sprintf("Ещё %d — по кнопке «Все избранные локации».", len(items)-visitorFavoritePreview)
	}
	return text
}
func (b *Bot) visitorAlertHTML(p visitorProfile, now time.Time, items []visitorFavorite) string {
	// Keep the alert within Telegram's text limit even with a long first route.
	// The visitor card retains the full route; all favorites have their own pages.
	summary := cloneVisitor(p)
	summary.FirstName = limitCaptionUnits(p.FirstName, 80)
	summary.LastName = limitCaptionUnits(p.LastName, 80)
	summary.Username = limitCaptionUnits(p.Username, 40)
	summary.FirstRoute = summary.FirstRoute[:min(10, len(summary.FirstRoute))]
	for i := range summary.FirstRoute {
		summary.FirstRoute[i].Name = limitCaptionUnits(summary.FirstRoute[i].Name, 80)
	}
	profile := visitorHTML(summary, now)
	if len(p.FirstRoute) > len(summary.FirstRoute) {
		profile += fmt.Sprintf("\n… ещё %d шагов — в карточке посетителя.", len(p.FirstRoute)-len(summary.FirstRoute))
	}
	return "🔔 <b>Посетитель без заявки · прошло 60 минут</b>\n\n" + visitorFavoritesPreview(items) + "\n\n" + profile
}
func (b *Bot) visitorAlertKeyboard(p visitorProfile) map[string]any {
	rows := visitorKeyboard(p)["inline_keyboard"].([][]Button)
	rows = append(rows, []Button{callbackButton("⭐ Все избранные локации", fmt.Sprintf("adm:visitorfavs:%d:0", p.UserID))})
	return markup(rows...)
}
func (b *Bot) showVisitorFavorites(ctx context.Context, adminID, userID int64, page int) error {
	if !b.isAdmin(adminID) || b.visitors == nil {
		return nil
	}
	p, ok := b.visitors.find(userID)
	if !ok || p.Hidden {
		return b.Send(ctx, adminID, "Посетитель не найден.", markup([]Button{callbackButton("⬅️ Посетители", "adm:visitors")}))
	}
	items := b.visitorFavorites(userID)
	pages := max(1, (len(items)+visitorFavoritePageSize-1)/visitorFavoritePageSize)
	page = max(0, min(page, pages-1))
	name := strings.TrimSpace(p.FirstName + " " + p.LastName)
	if name == "" {
		name = "ID " + strconv.FormatInt(userID, 10)
	}
	text := fmt.Sprintf("⭐ <b>Избранное посетителя</b>\n%s\nВсего: %d · Страница %d/%d\n\n", html.EscapeString(limitCaptionUnits(name, 120)), len(items), page+1, pages)
	if len(items) == 0 {
		text += "Ничего не добавлено в избранное."
	} else {
		text += visitorFavoriteLines(items[page*visitorFavoritePageSize : min((page+1)*visitorFavoritePageSize, len(items))])
	}
	rows := [][]Button{}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️ Назад", fmt.Sprintf("adm:visitorfavs:%d:%d", userID, page-1)))
	}
	if page+1 < pages {
		nav = append(nav, callbackButton("Далее ▶️", fmt.Sprintf("adm:visitorfavs:%d:%d", userID, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{callbackButton("👤 Карточка посетителя", fmt.Sprintf("adm:visitor:%d", userID))})
	return b.Send(ctx, adminID, text, markup(rows...))
}
