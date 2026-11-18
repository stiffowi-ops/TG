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

func cabinetButton() Button {
	return Button{"text": "📋 Мои заявки", "callback_data": "cab:home", "style": "success"}
}

func (b *Bot) homeLeadAction(userID int64) Button {
	if b.leads != nil {
		if lead, ok := b.leads.submitted(userID); ok && !lead.isResolved() {
			if lead.supplementLimitReached() {
				return caseButton("📂 Посмотреть заявку", lead.Number)
			}
			data := fmt.Sprintf("cab:add:%d", lead.Number)
			if draft, ok := b.leads.supplement(userID); ok && draft.Number == lead.Number {
				data = supplementData("resume", draft, "")
			}
			return callbackButton("➕ Дополнить заявку", data)
		}
		if _, ok := b.leads.draft(userID); ok {
			return callbackButton("▶️ Продолжить заявку", "lead:resume")
		}
	}
	return leadButton("menu", "")
}
func caseButton(title string, n int64) Button {
	return callbackButton(title, fmt.Sprintf("cab:view:%d", n))
}
func caseTime(at time.Time) string { return at.In(visitTZ).Format("02.01.2006 15:04") + " МСК" }
func supplementUsageText(l leadApplication) string {
	used := len(l.Updates)
	if used > maxCaseSupplements {
		used = maxCaseSupplements
	}
	text := fmt.Sprintf("➕ Дополнения: %d из %d использовано.", used, maxCaseSupplements)
	if !l.isResolved() && len(l.Updates) > 0 {
		latest := l.Updates[len(l.Updates)-1]
		if latest.AcceptedAt == nil {
			text += "\n⏳ Последнее дополнение ожидает принятия специалистом."
		} else {
			text += "\n✅ Последнее дополнение принято специалистом."
		}
	}
	return text
}

func locationLines(locations []leadLocation) string {
	var out strings.Builder
	for _, l := range locations {
		out.WriteString("• " + html.EscapeString(limitCaptionUnits(publicText(l.Name), 70)) + "\n")
	}
	return out.String()
}
func (b *Bot) showCabinet(ctx context.Context, chatID, userID int64) error {
	if chatID != userID || b.leads == nil {
		return b.Send(ctx, chatID, "Раздел «Мои заявки» временно недоступен.", homeMarkup())
	}
	if err := b.pauseClientForms(userID); err != nil {
		return err
	}
	text := "📋 <b>Мои заявки</b>\n\nЗдесь Вы можете проверить статус обращения, добавить интересующие локации или отправить уточнение. Предыдущие заявки всегда доступны в истории."
	rows := [][]Button{}
	if l, ok := b.leads.submitted(userID); ok && !l.isResolved() {
		text += fmt.Sprintf("\n\n<b>Активное обращение №%04d</b>\n%s\nЛокаций: %d\n%s", l.Number, leadStatus(l), len(l.locations()), supplementUsageText(l))
		if l.supplementLimitReached() {
			text += "\n" + supplementLimitText
		}
		rows = append(rows, []Button{caseButton("📂 Открыть обращение", l.Number)})
	} else {
		text += "\n\nАктивных обращений нет."
		rows = append(rows, []Button{leadButton("menu", "")})
	}
	if _, ok := b.leads.draft(userID); ok {
		rows = append(rows, []Button{callbackButton("▶️ Продолжить заполнение заявки", "lead:resume")})
	}
	if d, ok := b.leads.supplement(userID); ok {
		if l, exists := b.leads.byNumber(d.Number); exists && !l.isResolved() && !l.supplementLimitReached() {
			rows = append(rows, []Button{callbackButton("▶️ Продолжить дополнение", supplementData("resume", d, ""))})
		}
	}
	rows = append(rows, []Button{callbackButton("✅ Решённые обращения", "cab:resolved:0"), callbackButton("🕓 История обращений", "cab:archive:0")}, []Button{menuButton()})
	return b.Send(ctx, chatID, text, markup(rows...))
}
func (b *Bot) ownCase(userID, number int64) (leadApplication, bool) {
	l, ok := b.leads.byNumber(number)
	return l, ok && l.UserID == userID
}
func (b *Bot) showClientCase(ctx context.Context, chatID, userID, number int64) error {
	l, ok := b.ownCase(userID, number)
	if !ok {
		return b.Send(ctx, chatID, "Обращение недоступно.", markup([]Button{cabinetButton()}))
	}
	active, hasActive := b.leads.submitted(userID)
	editable := hasActive && active.Number == number && !l.isResolved()
	text := fmt.Sprintf("📂 <b>Обращение №%04d</b>\n%s\nПолучено: %s\n%s\n\n<b>Интересующие локации</b>\n%s", l.Number, leadStatus(l), caseTime(l.CreatedAt), supplementUsageText(l), locationLines(l.locations()))
	if l.TakenAt != nil {
		text += "\nВ работе с " + caseTime(*l.TakenAt) + ".\n"
	}
	if len(l.Updates) > 0 {
		u := l.Updates[len(l.Updates)-1]
		text += fmt.Sprintf("\n<b>Последнее дополнение №%d</b> · %s\n", u.ID, caseTime(u.CreatedAt))
		if u.Comment != "" && !l.isResolved() {
			text += html.EscapeString(limitCaptionUnits(u.Comment, 280)) + "\n"
		}
		if u.AcceptedAt != nil {
			text += "✅ Специалист принял дополнение.\n"
		} else if !l.isResolved() {
			text += "⏳ Ожидает принятия специалистом.\n"
		}
	}
	if l.isResolved() {
		if l.Resolution != "" {
			text += "\n<b>Итог</b>\n" + html.EscapeString(l.Resolution)
		}
		text += "\n" + processedText + " " + caseTime(*l.ResolvedAt)
	}
	rows := [][]Button{}
	if editable && l.supplementLimitReached() {
		text += "\n" + supplementLimitText
	} else if editable {
		rows = append(rows, []Button{callbackButton("➕ Добавить локацию", fmt.Sprintf("cab:add:%d", number))}, []Button{callbackButton("💬 Написать уточнение", fmt.Sprintf("cab:comment:%d", number))})
	} else if !l.isResolved() {
		text += "\nЭто предыдущее обращение из архива. Для дополнений откройте текущую заявку в разделе «Мои заявки»."
	}
	rows = append(rows, []Button{callbackButton("🕓 История этого обращения", fmt.Sprintf("cab:timeline:%d:0", number))})
	if l.isResolved() && (!hasActive || active.isResolved()) {
		rows = append(rows, []Button{leadButton("menu", "")})
	}
	rows = append(rows, []Button{cabinetButton(), menuButton()})
	return b.Send(ctx, chatID, text, markup(rows...))
}
func (b *Bot) showClientArchive(ctx context.Context, chatID, userID int64, page int, resolvedOnly bool) error {
	records := []leadApplication{}
	for _, l := range b.leads.allSubmitted() {
		if l.UserID == userID && (!resolvedOnly || l.isResolved()) {
			records = append(records, l)
		}
	}
	pages := max(1, (len(records)+7)/8)
	page = max(0, min(page, pages-1))
	title := "🕓 История обращений"
	kind := "archive"
	if resolvedOnly {
		title = "✅ Решённые обращения"
		kind = "resolved"
	}
	text := fmt.Sprintf("<b>%s</b>\n\nСтраница %d/%d", title, page+1, pages)
	rows := [][]Button{}
	for _, l := range records[page*8 : min((page+1)*8, len(records))] {
		rows = append(rows, []Button{caseButton(fmt.Sprintf("№%04d · %s", l.Number, leadStatus(l)), l.Number)})
	}
	if len(records) == 0 {
		text += "\n\nОбращений пока нет."
	}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️ Назад", fmt.Sprintf("cab:%s:%d", kind, page-1)))
	}
	if page+1 < pages {
		nav = append(nav, callbackButton("Далее ▶️", fmt.Sprintf("cab:%s:%d", kind, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{cabinetButton()})
	return b.Send(ctx, chatID, text, markup(rows...))
}

type caseEvent struct {
	at   time.Time
	text string
}

func caseEvents(l leadApplication) []caseEvent {
	events := []caseEvent{{l.CreatedAt, "📨 Обращение получено.\n" + locationLines([]leadLocation{{ID: l.LocationID, Name: l.LocationName}})}}
	if l.TakenAt != nil {
		events = append(events, caseEvent{*l.TakenAt, "🛠 Специалист взял обращение в работу."})
	}
	for _, u := range l.Updates {
		text := fmt.Sprintf("➕ Дополнение №%d\n", u.ID) + locationLines(u.Locations)
		if u.Comment != "" {
			text += html.EscapeString(u.Comment)
		}
		events = append(events, caseEvent{u.CreatedAt, text})
		if u.AcceptedAt != nil {
			events = append(events, caseEvent{*u.AcceptedAt, fmt.Sprintf("✅ Специалист принял дополнение №%d.", u.ID)})
		}
	}
	if l.ResolvedAt != nil {
		text := processedText
		if l.Resolution != "" {
			text += "\n" + html.EscapeString(l.Resolution)
		}
		events = append(events, caseEvent{*l.ResolvedAt, text})
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.After(events[j].at) })
	return events
}
func (b *Bot) showCaseTimeline(ctx context.Context, chatID int64, l leadApplication, page int, admin bool) error {
	events := caseEvents(l)
	if !admin {
		// Clients see the lifecycle in chronological order, with several short
		// events together rather than one event per page.
		for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
			events[i], events[j] = events[j], events[i]
		}
	}
	// Group by approximate message size as well as event count. A single
	// long legacy event stays intact and gets its own page.
	var chunks [][]caseEvent
	if admin {
		for _, event := range events {
			chunks = append(chunks, []caseEvent{event})
		}
	} else {
		var chunk []caseEvent
		length := 0
		for _, event := range events {
			line := "\n<b>" + caseTime(event.at) + "</b>\n" + event.text + "\n"
			if len(chunk) > 0 && (len(chunk) >= 4 || length+utf16Len(line) > 3500) {
				chunks = append(chunks, chunk)
				chunk = nil
				length = 0
			}
			chunk = append(chunk, event)
			length += utf16Len(line)
		}
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}
	}
	pages := max(1, len(chunks))
	page = max(0, min(page, pages-1))
	text := fmt.Sprintf("🕓 <b>История обращения №%04d</b>\nСтраница %d/%d\n", l.Number, page+1, pages)
	if !admin {
		text = fmt.Sprintf("🕓 <b>История обращения №%04d</b>\nСобытия по порядку · Страница %d/%d\n", l.Number, page+1, pages)
	}
	if len(chunks) > 0 {
		for _, e := range chunks[page] {
			text += "\n<b>" + caseTime(e.at) + "</b>\n" + e.text + "\n"
		}
	}
	prefix := "cab:timeline"
	back := caseButton("⬅️ К обращению", l.Number)
	if admin {
		prefix = "adm:case:timeline"
		back = callbackButton("⬅️ К обращению", fmt.Sprintf("adm:case:view:%d", l.Number))
	}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️ Назад", fmt.Sprintf("%s:%d:%d", prefix, l.Number, page-1)))
	}
	if page+1 < pages {
		nav = append(nav, callbackButton("Далее ▶️", fmt.Sprintf("%s:%d:%d", prefix, l.Number, page+1)))
	}
	rows := [][]Button{}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{back})
	return b.Send(ctx, chatID, text, markup(rows...))
}

func supplementData(action string, d supplementDraft, extra string) string {
	value := fmt.Sprintf("cab:%s:%d:%s", action, d.Number, d.Token)
	if extra != "" {
		value += ":" + extra
	}
	return value
}
func (b *Bot) beginSupplement(ctx context.Context, chatID, userID, number int64, mode, suggestedID string) error {
	l, ok := b.ownCase(userID, number)
	active, exists := b.leads.submitted(userID)
	if !ok || !exists || active.Number != number || l.isResolved() {
		return b.Send(ctx, chatID, "Дополнить можно текущее активное обращение.", markup([]Button{cabinetButton()}))
	}
	if err := l.supplementError(time.Now()); err != nil {
		return b.Send(ctx, chatID, err.Error(), markup([]Button{caseButton("📂 Открыть заявку", number)}))
	}
	if err := b.pauseClientForms(userID); err != nil {
		return err
	}
	if d, exists := b.leads.supplement(userID); exists && d.Number == number {
		return b.Send(ctx, chatID, "У Вас уже есть неотправленное дополнение. Продолжите его или отмените перед созданием нового.", markup(
			[]Button{callbackButton("▶️ Продолжить", supplementData("resume", d, ""))},
			[]Button{callbackButton("❌ Отменить дополнение", supplementData("cancel", d, ""))}, []Button{caseButton("⬅️ К обращению", number)}))
	}
	d, err := newSupplement(number)
	if err != nil {
		return err
	}
	if mode == "comment" {
		d.Stage = "comment"
	}
	if suggestedID != "" {
		for _, p := range l.locations() {
			if p.ID == suggestedID {
				return b.Send(ctx, chatID, "Эта локация уже есть в Вашем обращении.", markup([]Button{caseButton("📂 Открыть обращение", number)}))
			}
		}
		p, found := b.store.Find(suggestedID)
		if !found {
			return b.Send(ctx, chatID, "Локация недоступна.", markup([]Button{caseButton("⬅️ К обращению", number)}))
		}
		d.Locations = []leadLocation{{ID: p.ID, Name: publicText(p.Title)}}
		d.Stage = "review"
	}
	if err := b.leads.putSupplement(userID, d); err != nil {
		return err
	}
	return b.showSupplement(ctx, chatID, userID, d)
}
func (b *Bot) showSupplement(ctx context.Context, chatID, userID int64, d supplementDraft) error {
	text := fmt.Sprintf("➕ <b>Дополнение к обращению №%04d</b>\nК одной заявке можно отправить два дополнения.\n\n", d.Number)
	rows := [][]Button{}
	switch d.Stage {
	case "comment":
		text += "Напишите уточнение одним сообщением — до 1000 символов. Оно будет отправлено после Вашего подтверждения."
		rows = append(rows, []Button{callbackButton("⬅️ К проверке дополнения", supplementData("review", d, ""))})
	case "review":
		if len(d.Locations) > 0 {
			text += "<b>Добавляемые локации</b>\n" + locationLines(d.Locations)
		}
		if d.Comment != "" {
			text += "\n<b>Уточнение</b>\n" + html.EscapeString(d.Comment) + "\n"
		}
		if len(d.Locations) == 0 && d.Comment == "" {
			text += "Добавьте локацию или напишите уточнение.\n"
		} else {
			rows = append(rows, []Button{callbackButton("📨 Отправить дополнение", supplementData("send", d, ""))})
		}
		rows = append(rows, []Button{callbackButton("➕ Выбрать локации", supplementData("choose", d, ""))}, []Button{callbackButton("💬 Написать / изменить уточнение", supplementData("note", d, ""))})
	default:
		locations := sortedStations(b.store.List())
		pages := max(1, (len(locations)+9)/10)
		d.Page = max(0, min(d.Page, pages-1))
		text += fmt.Sprintf("Выберите несколько локаций. В дополнении: %d.\nСтраница %d/%d\n", len(d.Locations), d.Page+1, pages)
		existing := map[string]bool{}
		selected := map[string]bool{}
		l, _ := b.ownCase(userID, d.Number)
		for _, p := range l.locations() {
			existing[p.ID] = true
		}
		for _, p := range d.Locations {
			selected[p.ID] = true
		}
		for _, p := range locations[d.Page*10 : min((d.Page+1)*10, len(locations))] {
			mark := "☐ "
			if selected[p.ID] {
				mark = "✅ "
			}
			if existing[p.ID] {
				mark = "📌 "
			}
			rows = append(rows, []Button{callbackButton(mark+buttonLabel(p.Title), supplementData("pick", d, p.ID))})
		}
		text += "\n📌 — уже есть в обращении.\nМожно отправить часть названия сообщением для поиска."
		nav := []Button{}
		if d.Page > 0 {
			nav = append(nav, callbackButton("◀️ Назад", supplementData("page", d, strconv.Itoa(d.Page-1))))
		}
		if d.Page+1 < pages {
			nav = append(nav, callbackButton("Далее ▶️", supplementData("page", d, strconv.Itoa(d.Page+1))))
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
		rows = append(rows, []Button{callbackButton("✅ Проверить дополнение", supplementData("review", d, ""))})
	}
	rows = append(rows, []Button{callbackButton("❌ Отменить дополнение", supplementData("cancel", d, ""))}, []Button{caseButton("⬅️ К обращению", d.Number), cabinetButton()})
	return b.Send(ctx, chatID, text, markup(rows...))
}

func (b *Bot) handleCabinetCallback(ctx context.Context, chatID int64, from user, data string) error {
	if chatID != from.ID || from.ID <= 0 {
		return nil
	}
	if b.leads == nil {
		return b.Send(ctx, chatID, "Раздел «Мои заявки» временно недоступен.", homeMarkup())
	}
	p := strings.Split(data, ":")
	if len(p) < 2 {
		return nil
	}
	if p[1] == "home" {
		return b.showCabinet(ctx, chatID, from.ID)
	}
	if len(p) < 3 {
		return nil
	}
	number, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return nil
	}
	switch p[1] {
	case "resolved", "archive":
		if err := b.pauseClientForms(from.ID); err != nil {
			return err
		}
		return b.showClientArchive(ctx, chatID, from.ID, int(number), p[1] == "resolved")
	case "view":
		if err := b.pauseClientForms(from.ID); err != nil {
			return err
		}
		return b.showClientCase(ctx, chatID, from.ID, number)
	case "timeline":
		if err := b.pauseClientForms(from.ID); err != nil {
			return err
		}
		l, ok := b.ownCase(from.ID, number)
		if !ok {
			return b.showCabinet(ctx, chatID, from.ID)
		}
		page := 0
		if len(p) > 3 {
			page, _ = strconv.Atoi(p[3])
		}
		return b.showCaseTimeline(ctx, chatID, l, page, false)
	case "add", "comment":
		suggested := ""
		if len(p) > 3 {
			suggested = p[3]
		}
		return b.beginSupplement(ctx, chatID, from.ID, number, p[1], suggested)
	}
	if len(p) < 4 {
		return nil
	}
	token := p[3]
	if p[1] == "send" {
		if len(b.adminIDs) == 0 {
			return b.Send(ctx, chatID, "Передача дополнений временно недоступна. Черновик сохранён; попробуйте позже.", markup([]Button{cabinetButton()}))
		}
		l, changed, e := b.leads.submitSupplement(from.ID, number, token)
		if e != nil {
			return b.Send(ctx, chatID, html.EscapeString(e.Error()), markup([]Button{caseButton("📂 К обращению", number), cabinetButton()}))
		}
		text := "Это дополнение уже отправлено."
		if changed {
			text = "✅ Дополнение отправлено. Можно отправить ещё одно дополнение к этой заявке."
		}
		if l.supplementLimitReached() && !l.isResolved() {
			text = "✅ Дополнение отправлено. " + supplementLimitText
		}
		replyErr := b.Send(ctx, chatID, text, markup([]Button{caseButton("📂 Открыть обращение", l.Number)}))
		if changed {
			b.queueLeadDelivery(l.Number)
		}
		return replyErr
	}
	l, ok := b.ownCase(from.ID, number)
	if !ok || l.isResolved() {
		return b.showCabinet(ctx, chatID, from.ID)
	}
	if err := l.supplementError(time.Now()); err != nil {
		return b.Send(ctx, chatID, err.Error(), markup([]Button{cabinetButton()}))
	}
	d, ok := b.leads.supplement(from.ID)
	if !ok || d.Number != number || d.Token != token {
		return b.Send(ctx, chatID, "Этот экран дополнения устарел. Откройте обращение заново.", markup([]Button{cabinetButton()}))
	}
	if p[1] == "cancel" {
		if err := b.leads.cancelSupplement(from.ID, token); err != nil {
			return err
		}
		return b.showClientCase(ctx, chatID, from.ID, number)
	}
	d.Paused = false
	switch p[1] {
	case "resume":
	case "choose":
		d.Stage = "choose"
	case "note":
		d.Stage = "comment"
	case "review":
		d.Stage = "review"
	case "page":
		if len(p) != 5 {
			return nil
		}
		page, e := strconv.Atoi(p[4])
		if e != nil || page < 0 {
			return nil
		}
		d.Page = page
		d.Stage = "choose"
	case "pick":
		if len(p) != 5 {
			return nil
		}
		id := p[4]
		for _, v := range l.locations() {
			if v.ID == id {
				return b.showSupplement(ctx, chatID, from.ID, d)
			}
		}
		found := false
		for i, v := range d.Locations {
			if v.ID == id {
				d.Locations = append(d.Locations[:i], d.Locations[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			loc, exists := b.store.Find(id)
			if !exists {
				return b.showSupplement(ctx, chatID, from.ID, d)
			}
			if len(d.Locations) >= 10 || len(l.locations())+len(d.Locations) >= 20 {
				return b.Send(ctx, chatID, "Можно выбрать до 10 локаций за одно дополнение и до 20 на обращение.", markup([]Button{callbackButton("⬅️ К выбору", supplementData("choose", d, ""))}))
			}
			d.Locations = append(d.Locations, leadLocation{ID: id, Name: publicText(loc.Title)})
		}
		d.Stage = "choose"
	default:
		return nil
	}
	if err := b.leads.putSupplement(from.ID, d); err != nil {
		return err
	}
	return b.showSupplement(ctx, chatID, from.ID, d)
}

func (b *Bot) handleSupplementText(ctx context.Context, chatID int64, from user, input string) (bool, error) {
	if b.leads == nil || chatID != from.ID {
		return false, nil
	}
	d, ok := b.leads.supplement(from.ID)
	if !ok || d.Paused || strings.HasPrefix(input, "/") {
		return false, nil
	}
	l, ok := b.ownCase(from.ID, d.Number)
	if !ok || l.isResolved() {
		if err := b.leads.cancelSupplement(from.ID, d.Token); err != nil {
			return true, err
		}
		return false, nil
	}
	if limit := l.supplementError(time.Now()); limit != nil {
		if err := b.leads.cancelSupplement(from.ID, d.Token); err != nil {
			return true, err
		}
		return true, b.Send(ctx, chatID, limit.Error(), markup([]Button{cabinetButton()}))
	}
	if d.Stage == "comment" {
		input = strings.TrimSpace(input)
		if input == "" || len([]rune(input)) > 1000 {
			return true, b.Send(ctx, chatID, "Напишите уточнение от 1 до 1000 символов.", markup([]Button{callbackButton("⬅️ К дополнению", supplementData("review", d, ""))}))
		}
		d.Comment = input
		d.Stage = "review"
		if err := b.leads.putSupplement(from.ID, d); err != nil {
			return true, err
		}
		return true, b.showSupplement(ctx, chatID, from.ID, d)
	}
	if d.Stage == "choose" {
		matches := inlineLocationMatches(b.store.List(), input, 10)
		rows := [][]Button{}
		for _, p := range matches {
			rows = append(rows, []Button{callbackButton(buttonLabel(p.Title), supplementData("pick", d, p.ID))})
		}
		text := "Выберите локацию из результатов поиска:"
		if len(matches) == 0 {
			text = "Совпадений нет. Попробуйте другую часть названия."
		}
		rows = append(rows, []Button{callbackButton("📋 К списку локаций", supplementData("choose", d, "")), callbackButton("✅ Проверить", supplementData("review", d, ""))})
		return true, b.Send(ctx, chatID, text, markup(rows...))
	}
	return true, b.showSupplement(ctx, chatID, from.ID, d)
}
