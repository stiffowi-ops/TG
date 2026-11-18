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

	"locationbot/internal/catalog"
)

const leadPageSize = 10

func leadButton(source, id string) Button {
	data := "lead:open:" + source
	if id != "" {
		data += ":" + id
	}
	return callbackButton("📝 Оставить заявку", data)
}

// A selected inline card may have only inline_message_id. Start the form in
// the clicking user's private chat; never post private details into a group.
func (b *Bot) openInlineLead(ctx context.Context, callback *callbackQuery) error {
	if callback == nil || callback.From.ID <= 0 {
		return nil
	}
	id := strings.TrimPrefix(callback.Data, "inline:nav:lead:")
	if !catalog.ValidID(id) {
		return b.replyInlineCallback(ctx, callback, "Локация не найдена.")
	}
	if err := b.openLead(ctx, callback.From.ID, callback.From.ID, "card", id); err != nil {
		log.Printf("Lead form private open failed: %v", err)
		return b.replyInlineCallback(ctx, callback, "Откройте личный чат с ботом и нажмите /start.")
	}
	return b.replyInlineCallback(ctx, callback, "Форма заявки открыта в личном чате ✅")
}

func (b *Bot) openLead(ctx context.Context, chatID, userID int64, source, suggestedID string) error {
	if err := b.pauseClientForms(userID); err != nil {
		return err
	}
	if b.leads == nil || b.store == nil || b.favorites == nil || len(b.adminIDs) == 0 {
		return b.Send(ctx, chatID, "Приём заявок временно недоступен. Пожалуйста, попробуйте позже.", markup([]Button{menuButton()}))
	}
	if previous, exists := b.leads.submitted(userID); exists && !previous.isResolved() {
		if source == "card" && suggestedID != "" && !previous.supplementLimitReached() {
			return b.Send(ctx, chatID, fmt.Sprintf("У Вас есть активное обращение №%04d. Добавить выбранную локацию к нему?", previous.Number), markup([]Button{callbackButton("➕ Добавить к моему обращению", fmt.Sprintf("cab:add:%d:%s", previous.Number, suggestedID))}, []Button{caseButton("📂 Открыть обращение", previous.Number), cabinetButton()}))
		}
		return b.leadAlreadySent(ctx, chatID, previous)
	}
	if source == "favorites" && len(b.favoriteLocations(userID)) == 0 {
		return b.Send(ctx, chatID, "⭐ <b>В избранном пока нет локаций.</b>\n\nСначала сохраните интересующую локацию, затем вернитесь к заявке.", markup([]Button{callbackButton("📋 Список локаций", "list")}, []Button{menuButton()}))
	}
	if _, exists := b.leads.draft(userID); exists {
		return b.Send(ctx, chatID,
			"📝 <b>Ваша заявка ещё не отправлена.</b>\n\nВы можете продолжить заполнение с того места, на котором остановились, или начать заново.",
			markup([]Button{callbackButton("▶️ Продолжить", "lead:resume")}, []Button{callbackButton("🔄 Начать заново", "lead:restart:"+source+":"+suggestedID)}, []Button{menuButton()}))
	}
	return b.startLead(ctx, chatID, userID, source, suggestedID)
}

func (b *Bot) startLead(ctx context.Context, chatID, userID int64, source, suggestedID string) error {
	if source != "favorites" && source != "card" {
		source = "menu"
	}
	if source == "card" {
		if !catalog.ValidID(suggestedID) {
			return b.Send(ctx, chatID, "Не удалось определить выбранную локацию.", nil)
		}
		if _, ok := b.store.Find(suggestedID); !ok {
			return b.Send(ctx, chatID, "Локация больше не доступна.", nil)
		}
	} else {
		suggestedID = ""
	}
	draft := leadDraft{Source: source, SuggestedID: suggestedID, Stage: "kind"}
	if previous, ok := b.leads.submitted(userID); ok && previous.isResolved() {
		draft.Kind = previous.Kind
		draft.Company = previous.Company
		draft.Name = previous.Name
		draft.Phone = previous.Phone
		draft.Stage = "reuse"
	}
	if err := b.leads.putDraft(userID, draft); err != nil {
		return err
	}
	return b.showLead(ctx, chatID, userID, draft)
}

func (b *Bot) leadAlreadySent(ctx context.Context, chatID int64, previous leadApplication) error {
	if previous.supplementLimitReached() {
		return b.Send(ctx, chatID, supplementLimitText, markup([]Button{caseButton("📂 Открыть заявку", previous.Number), cabinetButton()}))
	}
	return b.Send(ctx, chatID, fmt.Sprintf("📂 <b>У Вас есть активное обращение №%04d</b>\n%s\n\nВы можете добавить локации и уточнения к этому обращению. После его завершения можно будет создать новое.", previous.Number, leadStatus(previous)), markup(
		[]Button{caseButton("📂 Открыть обращение", previous.Number)},
		[]Button{callbackButton("➕ Добавить локацию", fmt.Sprintf("cab:add:%d", previous.Number)), callbackButton("💬 Уточнить", fmt.Sprintf("cab:comment:%d", previous.Number))},
		[]Button{cabinetButton(), menuButton()},
	))
}

func leadNav(back bool) [][]Button {
	rows := make([][]Button, 0, 2)
	if back {
		rows = append(rows, []Button{callbackButton("⬅️ Назад", "lead:back")})
	}
	return append(rows, []Button{callbackButton("❌ Отменить заявку", "lead:cancel")})
}

func (b *Bot) showLead(ctx context.Context, chatID, userID int64, d leadDraft) error {
	switch d.Stage {
	case "reuse":
		text := "📝 <b>Новое обращение</b>\n\nИспользовать контактные данные предыдущего обращения?\n\n<b>Имя:</b> " + html.EscapeString(d.Name) + "\n<b>Телефон:</b> " + html.EscapeString(d.Phone)
		if d.Company != "" {
			text += "\n<b>Компания:</b> " + html.EscapeString(d.Company)
		}
		return b.Send(ctx, chatID, text, markup([]Button{callbackButton("✅ Использовать данные", "lead:reuse")}, []Button{callbackButton("✏️ Заполнить заново", "lead:fresh")}, []Button{callbackButton("❌ Отменить", "lead:cancel")}))
	case "kind":
		return b.Send(ctx, chatID, "📝 <b>Оставить заявку</b>\n\nДля начала уточним один момент: Вы уже сотрудничаете с нами?", markup(
			[]Button{callbackButton("🤝 Уже работаю с Вами", "lead:kind:existing")},
			[]Button{callbackButton("🚀 Хочу работать с Вами", "lead:kind:new")},
			[]Button{callbackButton("❌ Отменить", "lead:cancel")},
		))
	case "company":
		return b.Send(ctx, chatID, "📝 <b>Шаг 1 из 4</b>\n\nУкажите, пожалуйста, <b>полное название Вашего юридического лица</b>.\n\nНапишите название ответным сообщением.", markup(leadNav(true)...))
	case "name":
		step, total := 1, 3
		if d.Kind == "new" {
			step, total = 2, 4
		}
		return b.Send(ctx, chatID, fmt.Sprintf("📝 <b>Шаг %d из %d</b>\n\nКак к Вам обращаться? Напишите, пожалуйста, <b>Ваше имя</b>.", step, total), markup(leadNav(true)...))
	case "phone":
		step, total := 2, 3
		if d.Kind == "new" {
			step, total = 3, 4
		}
		return b.Send(ctx, chatID, fmt.Sprintf("📱 <b>Шаг %d из %d</b>\n\nУкажите <b>Ваш номер телефона</b> для связи.\nНомер должен начинаться с <b>+7</b> и содержать ещё <b>10 цифр</b>.\nНапример: <code>+7 999 123-45-67</code>.", step, total), markup(leadNav(true)...))
	case "choose", "search":
		return b.showLeadLocations(ctx, chatID, userID, d)
	case "location_confirm":
		loc, ok := b.store.Find(d.LocationID)
		if !ok {
			return b.Send(ctx, chatID, "Локация больше не доступна. Пожалуйста, выберите другую.", markup([]Button{callbackButton("🔎 Выбрать локацию", "lead:change")}))
		}
		return b.Send(ctx, chatID, "📍 <b>Подтверждение локации</b>\n\nВас интересует локация:\n\n<b>"+html.EscapeString(publicText(loc.Title))+"</b>?", markup(
			[]Button{callbackButton("✅ Подтвердить", "lead:confirm")},
			[]Button{callbackButton("🔄 Сменить локацию", "lead:change")},
			[]Button{callbackButton("⬅️ Назад", "lead:back"), callbackButton("❌ Отменить", "lead:cancel")},
		))
	case "review":
		loc, ok := b.store.Find(d.LocationID)
		if !ok {
			return b.Send(ctx, chatID, "Выбранная локация больше не доступна.", markup([]Button{callbackButton("🔄 Сменить локацию", "lead:change")}))
		}
		clientType := "Уже работаю с Вами"
		if d.Kind == "new" {
			clientType = "Хочу работать с Вами"
		}
		var details strings.Builder
		details.WriteString("✅ <b>Проверьте Вашу заявку</b>\n\n")
		details.WriteString("<b>Статус:</b> " + clientType + "\n")
		if d.Kind == "new" {
			details.WriteString("<b>Юридическое лицо:</b> " + html.EscapeString(d.Company) + "\n")
		}
		details.WriteString("<b>Имя:</b> " + html.EscapeString(d.Name) + "\n")
		details.WriteString("<b>Телефон:</b> " + html.EscapeString(d.Phone) + "\n")
		details.WriteString("<b>Локация:</b> " + html.EscapeString(publicText(loc.Title)) + "\n\n")
		details.WriteString("Отправляя заявку, Вы разрешаете использовать указанные контактные данные для связи с Вами по этому обращению.")
		return b.Send(ctx, chatID, details.String(), markup(
			[]Button{callbackButton("📨 Отправить заявку", "lead:submit")},
			[]Button{callbackButton("✏️ Изменить данные", "lead:edit"), callbackButton("🔄 Сменить локацию", "lead:change")},
			[]Button{callbackButton("❌ Отменить", "lead:cancel")},
		))
	default:
		return b.Send(ctx, chatID, "Откройте форму ещё раз из главного меню.", markup([]Button{menuButton()}))
	}
}

func (b *Bot) leadLocations(userID int64, source string) []catalog.Location {
	if b.store == nil {
		return nil
	}
	if source == "favorites" {
		return b.favoriteLocations(userID)
	}
	return sortedStations(b.store.List())
}

func (b *Bot) showLeadLocations(ctx context.Context, chatID, userID int64, d leadDraft) error {
	locations := b.leadLocations(userID, d.Source)
	if len(locations) == 0 {
		return b.Send(ctx, chatID, "⭐ <b>В избранном пока нет локаций.</b>\n\nСохраните интересующую локацию в избранное, затем вернитесь к заявке.", markup(
			[]Button{callbackButton("⬅️ Назад", "lead:back")}, []Button{callbackButton("❌ Отменить", "lead:cancel")},
		))
	}
	pages := (len(locations) + leadPageSize - 1) / leadPageSize
	if d.Page < 0 {
		d.Page = 0
	}
	if d.Page >= pages {
		d.Page = pages - 1
	}
	from := d.Page * leadPageSize
	until := from + leadPageSize
	if until > len(locations) {
		until = len(locations)
	}
	var rows [][]Button
	for i := from; i < until; i += 2 {
		row := []Button{callbackButton(buttonLabel(locations[i].Title), "lead:pick:"+locations[i].ID)}
		if i+1 < until {
			row = append(row, callbackButton(buttonLabel(locations[i+1].Title), "lead:pick:"+locations[i+1].ID))
		}
		rows = append(rows, row)
	}
	var pager []Button
	if d.Page > 0 {
		pager = append(pager, callbackButton("◀️ Назад", "lead:page:"+strconv.Itoa(d.Page-1)))
	}
	if d.Page+1 < pages {
		pager = append(pager, callbackButton("Далее ▶️", "lead:page:"+strconv.Itoa(d.Page+1)))
	}
	if len(pager) > 0 {
		rows = append(rows, pager)
	}
	rows = append(rows, []Button{callbackButton("🔎 Найти по названию", "lead:search")})
	rows = append(rows, []Button{callbackButton("⬅️ К телефону", "lead:back"), callbackButton("❌ Отменить", "lead:cancel")})
	subtitle := "Выберите интересующую Вас локацию из каталога."
	if d.Source == "favorites" {
		subtitle = "Выберите локацию из Вашего избранного."
	}
	return b.Send(ctx, chatID, fmt.Sprintf("📍 <b>Выбор локации</b>\n\n%s\nСтраница %d из %d", subtitle, d.Page+1, pages), markup(rows...))
}

func (b *Bot) handleLeadCallback(ctx context.Context, chatID int64, from user, data string) error {
	if strings.HasPrefix(data, "lead:open:") {
		raw := strings.TrimPrefix(data, "lead:open:")
		parts := strings.SplitN(raw, ":", 2)
		id := ""
		if len(parts) > 1 {
			id = parts[1]
		}
		return b.openLead(ctx, chatID, from.ID, parts[0], id)
	}
	if b.leads == nil || b.store == nil {
		return b.Send(ctx, chatID, "Сейчас форма недоступна.", nil)
	}
	if prev, ok := b.leads.submitted(from.ID); ok && !prev.isResolved() {
		return b.leadAlreadySent(ctx, chatID, prev)
	}
	if strings.HasPrefix(data, "lead:restart:") {
		raw := strings.TrimPrefix(data, "lead:restart:")
		parts := strings.SplitN(raw, ":", 2)
		id := ""
		if len(parts) > 1 {
			id = parts[1]
		}
		return b.startLead(ctx, chatID, from.ID, parts[0], id)
	}
	d, ok := b.leads.draft(from.ID)
	if !ok {
		return b.Send(ctx, chatID, "Заявка ещё не начата. Нажмите «Оставить заявку».", markup([]Button{leadButton("menu", "")}))
	}
	d.Paused = false
	switch {
	case data == "lead:reuse" && d.Stage == "reuse":
		if d.Source == "card" && d.SuggestedID != "" {
			d.LocationID = d.SuggestedID
			d.Stage = "location_confirm"
		} else {
			d.Stage = "choose"
		}
	case data == "lead:fresh" && d.Stage == "reuse":
		d = leadDraft{Source: d.Source, SuggestedID: d.SuggestedID, Stage: "kind"}
	case data == "lead:resume":
		if err := b.leads.putDraft(from.ID, d); err != nil {
			return err
		}
		return b.showLead(ctx, chatID, from.ID, d)
	case data == "lead:cancel":
		if err := b.leads.clearDraft(from.ID); err != nil {
			return err
		}
		return b.Send(ctx, chatID, "Заявка отменена. Вы можете заполнить новую в любое время.", markup([]Button{menuButton()}))
	case strings.HasPrefix(data, "lead:kind:") && d.Stage == "kind":
		kind := strings.TrimPrefix(data, "lead:kind:")
		if kind != "existing" && kind != "new" {
			return nil
		}
		d.Kind = kind
		if kind == "new" {
			d.Stage = "company"
		} else {
			d.Stage = "name"
		}
	case data == "lead:change" && (d.Stage == "location_confirm" || d.Stage == "review"):
		d.Stage = "choose"
		d.Page = 0
		d.LocationID = ""
	case data == "lead:search" && (d.Stage == "choose" || d.Stage == "search"):
		d.Stage = "search"
		if err := b.leads.putDraft(from.ID, d); err != nil {
			return err
		}
		return b.Send(ctx, chatID, "🔎 <b>Поиск локации для заявки</b>\n\nНапишите название или его часть. Я покажу варианты для выбора.", markup([]Button{callbackButton("📋 Посмотреть список", "lead:show-list")}, []Button{callbackButton("❌ Отменить", "lead:cancel")}))
	case data == "lead:show-list" && d.Stage == "search":
		d.Stage = "choose"
		d.Page = 0
	case strings.HasPrefix(data, "lead:page:") && (d.Stage == "choose" || d.Stage == "search"):
		page, err := strconv.Atoi(strings.TrimPrefix(data, "lead:page:"))
		if err != nil || page < 0 {
			return nil
		}
		d.Stage = "choose"
		d.Page = page
	case strings.HasPrefix(data, "lead:pick:") && (d.Stage == "choose" || d.Stage == "search"):
		id := strings.TrimPrefix(data, "lead:pick:")
		if !catalog.ValidID(id) {
			return nil
		}
		if _, found := b.store.Find(id); !found {
			return b.Send(ctx, chatID, "Локация не найдена. Выберите другую.", nil)
		}
		if d.Source == "favorites" && !b.favorites.Contains(from.ID, id) {
			return b.Send(ctx, chatID, "Этой локации больше нет в Вашем избранном.", nil)
		}
		d.LocationID = id
		d.Stage = "location_confirm"
	case data == "lead:confirm" && d.Stage == "location_confirm":
		if d.LocationID == "" {
			return nil
		}
		d.Stage = "review"
	case data == "lead:edit" && d.Stage == "review":
		if d.Kind == "new" {
			d.Stage = "company"
		} else {
			d.Stage = "name"
		}
	case data == "lead:back":
		switch d.Stage {
		case "company":
			d.Stage = "kind"
		case "name":
			if d.Kind == "new" {
				d.Stage = "company"
			} else {
				d.Stage = "kind"
			}
		case "phone":
			d.Stage = "name"
		case "choose", "search":
			d.Stage = "phone"
		case "location_confirm":
			d.Stage = "choose"
		case "review":
			d.Stage = "location_confirm"
		default:
			return nil
		}
	case data == "lead:submit" && d.Stage == "review":
		return b.submitLead(ctx, chatID, from, d)
	default:
		return b.showLead(ctx, chatID, from.ID, d)
	}
	if err := b.leads.putDraft(from.ID, d); err != nil {
		return err
	}
	return b.showLead(ctx, chatID, from.ID, d)
}

func (b *Bot) handleLeadText(ctx context.Context, chatID int64, from user, input string) (bool, error) {
	if b.leads == nil {
		return false, nil
	}
	d, ok := b.leads.draft(from.ID)
	if !ok || d.Paused {
		return false, nil
	}
	// Allow commands to exit the form without swallowing /start, /id, etc.
	if strings.HasPrefix(input, "/") {
		return false, nil
	}
	input = strings.TrimSpace(input)
	switch d.Stage {
	case "company":
		if len([]rune(input)) < 3 || len([]rune(input)) > 180 {
			return true, b.Send(ctx, chatID, "Укажите, пожалуйста, полное название юридического лица (от 3 до 180 символов).", markup(leadNav(true)...))
		}
		d.Company = input
		d.Stage = "name"
	case "name":
		if len([]rune(input)) < 2 || len([]rune(input)) > 100 {
			return true, b.Send(ctx, chatID, "Укажите, пожалуйста, Ваше имя (от 2 до 100 символов).", markup(leadNav(true)...))
		}
		d.Name = input
		d.Stage = "phone"
	case "phone":
		phone, ok := normalizeLeadPhone(input)
		if !ok {
			return true, b.Send(ctx, chatID, "Проверьте номер телефона. Он должен начинаться с <b>+7</b> и содержать ровно <b>10 цифр после +7</b>. Например: <code>+7 999 123-45-67</code>. Пробелы, скобки и дефисы допускаются.", markup(leadNav(true)...))
		}
		d.Phone = phone
		if d.Source == "card" && d.SuggestedID != "" {
			d.LocationID = d.SuggestedID
			d.Stage = "location_confirm"
		} else {
			d.Stage = "choose"
			d.Page = 0
		}
	case "search":
		matches := catalog.Search(b.leadLocations(from.ID, d.Source), input)
		matches = sortedStations(matches)
		if len(matches) == 0 {
			return true, b.Send(ctx, chatID, "По этому запросу локаций не найдено. Попробуйте другую часть названия.", markup([]Button{callbackButton("📋 Список локаций", "lead:show-list")}))
		}
		rows := make([][]Button, 0, 10)
		for i := 0; i < len(matches) && i < 12; i++ {
			rows = append(rows, []Button{callbackButton(buttonLabel(matches[i].Title), "lead:pick:"+matches[i].ID)})
		}
		rows = append(rows, []Button{callbackButton("📋 Список локаций", "lead:show-list")})
		return true, b.Send(ctx, chatID, "🔎 <b>Подходящие локации</b>\n\nВыберите нужный вариант:", markup(rows...))
	default:
		return true, b.showLead(ctx, chatID, from.ID, d)
	}
	if err := b.leads.putDraft(from.ID, d); err != nil {
		return true, err
	}
	return true, b.showLead(ctx, chatID, from.ID, d)
}

// normalizeLeadPhone accepts Russian phone numbers only: +7 followed by
// exactly 10 digits. Formatting separators are allowed but removed before
// storage, so the client and all administrators see a consistent +7XXXXXXXXXX.
func normalizeLeadPhone(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "+7") {
		return "", false
	}
	digits := make([]byte, 0, 11)
	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, byte(r))
		case r == '+' && i == 0:
			// Allowed only as the initial country-code sign.
		case r == ' ' || r == '-' || r == '(' || r == ')':
			// Cosmetic separators don't count towards the phone length.
		default:
			return "", false
		}
	}
	if len(digits) != 11 || digits[0] != '7' {
		return "", false
	}
	return "+" + string(digits), true
}

func validLeadPhone(value string) bool {
	_, ok := normalizeLeadPhone(value)
	return ok
}

func (b *Bot) submitLead(ctx context.Context, chatID int64, from user, d leadDraft) error {
	if len(b.adminIDs) == 0 {
		return b.Send(ctx, chatID, "Приём заявок временно недоступен. Пожалуйста, попробуйте позже.", markup([]Button{callbackButton("⬅️ К заявке", "lead:resume")}))
	}
	if d.Stage != "review" || (d.Kind != "new" && d.Kind != "existing") || !validLeadPhone(d.Phone) {
		return b.Send(ctx, chatID, "Проверьте заполнение заявки перед отправкой.", nil)
	}
	loc, ok := b.store.Find(d.LocationID)
	if !ok {
		return b.Send(ctx, chatID, "Выбранная локация больше не доступна. Измените её.", markup([]Button{callbackButton("🔄 Сменить локацию", "lead:change")}))
	}
	if d.Source == "favorites" && !b.favorites.Contains(from.ID, d.LocationID) {
		return b.Send(ctx, chatID, "Выбранной локации больше нет в Вашем избранном. Выберите другую.", markup([]Button{callbackButton("🔄 Сменить локацию", "lead:change")}))
	}
	submitted, already, err := b.leads.submit(from.ID, from.Username, publicText(loc.Title))
	if err != nil {
		return b.Send(ctx, chatID, "Не удалось сохранить заявку. Пожалуйста, попробуйте отправить её ещё раз.", markup([]Button{callbackButton("📨 Повторить отправку", "lead:submit")}))
	}
	if already {
		return b.leadAlreadySent(ctx, chatID, submitted)
	}
	// A successful application permanently clears the abandonment status;
	// existing one-time alerts stay in history but no new alert is sent.
	if b.visitors != nil {
		if err := b.visitors.markLead(from.ID); err != nil {
			log.Printf("Visitor %d status after application: %v", from.ID, err)
		}
	}
	replyErr := b.Send(ctx, chatID, fmt.Sprintf("✅ <b>Ваша заявка №%04d успешно отправлена!</b>\n\nСпасибо за интерес к нашим локациям!\n\nМы получили Вашу заявку. Наш специалист свяжется с Вами по указанному номеру телефона, чтобы обсудить детали сотрудничества.", submitted.Number), markup([]Button{caseButton("📂 Моё обращение", submitted.Number)}, []Button{cabinetButton(), menuButton()}))
	// Persisted once, deliver individually to every ADMIN_IDS recipient.
	// The customer confirmation is sent first so slow Telegram admin chats
	// do not hold up the visible success response.
	b.queueLeadDelivery(submitted.Number)
	return replyErr
}

func adminLeadText(lead leadApplication) string {
	kind := "Уже работаем вместе"
	if lead.Kind == "new" {
		kind = "Хочет работать с нами"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📨 <b>Заявка №%04d</b>\n\n", lead.Number))
	b.WriteString("<b>Тип:</b> " + kind + "\n")
	if lead.Kind == "new" {
		b.WriteString("<b>Юрлицо:</b> " + html.EscapeString(lead.Company) + "\n")
	}
	b.WriteString("<b>Имя:</b> " + html.EscapeString(lead.Name) + "\n")
	b.WriteString("<b>Телефон:</b> " + html.EscapeString(lead.Phone) + "\n")
	b.WriteString("<b>Локация:</b> " + html.EscapeString(lead.LocationName) + " (" + html.EscapeString(lead.LocationID) + ")\n")
	b.WriteString("<b>Telegram ID:</b> <code>" + strconv.FormatInt(lead.UserID, 10) + "</code>\n")
	if lead.Username != "" {
		b.WriteString("<b>Telegram:</b> @" + html.EscapeString(lead.Username) + "\n")
	}
	b.WriteString("<b>Источник:</b> " + html.EscapeString(lead.Source) + "\n")
	b.WriteString("<b>Дата:</b> " + caseTime(lead.CreatedAt))
	b.WriteString("\n<b>Статус:</b> " + leadStatus(lead))
	if lead.TakenBy > 0 {
		b.WriteString(fmt.Sprintf("\n<b>Ответственный:</b> <code>%d</code>", lead.TakenBy))
	}
	b.WriteString("\n\n<b>Все локации</b>\n" + locationLines(lead.locations()))
	pending := 0
	for _, u := range lead.Updates {
		if u.AcceptedAt == nil {
			pending++
		}
	}
	if lead.isResolved() && len(lead.Updates) > 0 {
		b.WriteString(fmt.Sprintf("\nДополнений: %d", len(lead.Updates)))
	} else if len(lead.Updates) > 0 {
		b.WriteString(fmt.Sprintf("\nДополнений: %d · Ожидают принятия: %d", len(lead.Updates), pending))
	}
	if lead.isResolved() && lead.Resolution != "" {
		b.WriteString("\n\n<b>Итог</b>\n" + html.EscapeString(limitCaptionUnits(lead.Resolution, 450)) + "\nПолный итог — в истории обращения.")
	}
	if lead.isResolved() && lead.ServiceVersion >= 2 {
		if lead.ResolvedBy == 0 {
			b.WriteString("\nЗавершена автоматически.")
		} else {
			b.WriteString("\nЗавершена администратором.")
		}
	}
	return b.String()
}

// The green "taken in work" button is available in recent Telegram clients.
// Its callback is intentionally inert after a successful transition.
func adminLeadMarkup(lead leadApplication) map[string]any {
	rows := [][]Button{{adminCaseButton(lead.Number)}}
	if lead.isResolved() {
		rows = append(rows, []Button{{"text": "✅ Решено", "callback_data": fmt.Sprintf("adm:case:view:%d", lead.Number), "style": "success"}})
	} else if lead.TakenInWork {
		rows = append(rows, []Button{{"text": "✅ Взято в работу", "callback_data": fmt.Sprintf("lead:done:%d", lead.Number), "style": "success"}}, []Button{callbackButton("✅ Завершить заявку", fmt.Sprintf("adm:case:resolve:%d", lead.Number))})
	} else {
		rows = append(rows, []Button{callbackButton("📥 Взять в работу", fmt.Sprintf("lead:take:%d", lead.Number))})
	}
	return markup(rows...)
}

func (b *Bot) notifyLeadAdmins(ctx context.Context, lead leadApplication) {
	if b.leads == nil {
		return
	}
	b.leadNotifyMu.Lock()
	defer b.leadNotifyMu.Unlock()
	ids := make([]int64, 0, len(b.adminIDs))
	for id := range b.adminIDs {
		if !lead.Delivered[strconv.FormatInt(id, 10)] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		// Read fresh state on each attempt: the lead might already be taken
		// into work while another admin notification is being sent.
		actual, exists := b.leads.byNumber(lead.Number)
		if !exists || actual.Hidden {
			return
		}
		key := strconv.FormatInt(id, 10)
		if actual.Delivered[key] {
			continue
		}
		raw, err := b.call(ctx, "sendMessage", map[string]any{
			"chat_id": id, "text": adminLeadText(actual), "parse_mode": "HTML",
			"link_preview_options": map[string]any{"is_disabled": true},
			"reply_markup":         adminLeadMarkup(actual),
		})
		if err != nil {
			log.Printf("Lead %d notify admin %d: %v", lead.Number, id, err)
			continue
		}
		var sent struct {
			MessageID int `json:"message_id"`
		}
		if err = json.Unmarshal(raw, &sent); err != nil {
			log.Printf("Lead delivery message receipt: %v", err)
		}
		if err := b.leads.markDelivered(lead.UserID, lead.Number, id, sent.MessageID); err != nil {
			log.Printf("Lead %d delivery receipt not saved: %v", lead.Number, err)
		}
	}
}

// Only numeric ADMIN_IDS may take an application into work. Old callback
// messages are safe to click again: the state transition is idempotent.
func (b *Bot) handleLeadAdminCallback(ctx context.Context, cb *callbackQuery) error {
	if cb == nil || cb.From.ID <= 0 || !b.adminIDs[cb.From.ID] || cb.Message == nil || cb.Message.Chat.Type != "private" || cb.Message.Chat.ID != cb.From.ID {
		return nil
	}
	raw := strings.TrimPrefix(strings.TrimPrefix(cb.Data, "lead:take:"), "lead:done:")
	number, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || number <= 0 {
		return nil
	}
	if strings.HasPrefix(cb.Data, "lead:take:") {
		_, _, err = b.leads.takeInWork(number, cb.From.ID)
		if err != nil {
			return b.Send(withNavigation(ctx, cb.From.ID, cb.Message.MessageID), cb.From.ID, "Не удалось изменить статус заявки: "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К заявкам", "adm:leads")}))
		}
	}
	lead, ok := b.leads.byNumber(number)
	if !ok {
		return nil
	}
	b.queueLeadDelivery(lead.Number)
	key := strconv.FormatInt(cb.From.ID, 10)
	if lead.AdminMessages[key] != cb.Message.MessageID {
		// Old notifications predate persistent message-ID tracking.
		if _, err := b.call(ctx, "editMessageReplyMarkup", map[string]any{
			"chat_id": cb.From.ID, "message_id": cb.Message.MessageID, "reply_markup": adminLeadMarkup(lead),
		}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
			log.Printf("Lead %d update clicked message: %v", number, err)
		}
	}
	return nil
}

// Synchronize all copies of a notification after a lead is taken, regardless
// of which administrative screen initiated the transition.
func (b *Bot) refreshLeadAdminMessages(ctx context.Context, lead leadApplication) {
	for id, mid := range lead.AdminMessages {
		adminID, err := strconv.ParseInt(id, 10, 64)
		if err != nil || !b.adminIDs[adminID] || mid <= 0 {
			continue
		}
		if _, err = b.call(ctx, "editMessageText", map[string]any{"chat_id": adminID, "message_id": mid, "text": adminLeadText(lead), "parse_mode": "HTML", "link_preview_options": map[string]any{"is_disabled": true}, "reply_markup": adminLeadMarkup(lead)}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
			log.Printf("Lead %d refresh admin %d: %v", lead.Number, adminID, err)
		}
	}
}

func (b *Bot) retryPendingLeadNotifications(ctx context.Context) {
	if b.leads == nil {
		return
	}
	for _, lead := range b.leads.visibleSubmitted() {
		b.notifyLeadAdmins(ctx, lead)
	}
	b.notifyCaseEvents(ctx, time.Now().UTC())
}

func (b *Bot) showAdminLeads(ctx context.Context, chatID int64) error {
	return b.showAdminLeadsPage(ctx, chatID, 0)
}
func (b *Bot) showAdminLeadsPage(ctx context.Context, chatID int64, page int) error {
	if b.leads == nil {
		return b.Send(ctx, chatID, "Хранилище заявок недоступно.", nil)
	}
	all := b.leads.visibleSubmitted()
	pages := max(1, (len(all)+7)/8)
	page = max(0, min(page, pages-1))
	pending, working, resolved := 0, 0, 0
	for _, l := range all {
		if l.isResolved() {
			resolved++
		} else if l.TakenInWork {
			working++
		} else {
			pending++
		}
	}
	text := fmt.Sprintf("📨 <b>Заявки</b>\nВсего: %d · Получено: %d · В работе: %d · Решено: %d\nСтраница %d/%d", len(all), pending, working, resolved, page+1, pages)
	rows := [][]Button{}
	for _, l := range all[page*8 : min((page+1)*8, len(all))] {
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("№%04d · %s · %s", l.Number, leadStatus(l), limitRunes(l.Name, 20)), fmt.Sprintf("adm:case:view:%d", l.Number))})
	}
	if len(all) == 0 {
		text += "\n\nЗаявок пока нет."
	}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️ Назад", fmt.Sprintf("adm:case:list:%d", page-1)))
	}
	if page+1 < pages {
		nav = append(nav, callbackButton("Далее ▶️", fmt.Sprintf("adm:case:list:%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{callbackButton("🗑 Выбрать заявки", "adm:clean:start:l:a"), callbackButton("♻️ Корзина", "adm:clean:start:l:t")}, []Button{callbackButton("⬅️ Админ-панель", "adm:menu")})
	return b.Send(ctx, chatID, text, markup(rows...))
}
