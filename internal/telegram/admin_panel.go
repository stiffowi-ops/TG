package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"locationbot/internal/adminbackup"
	"locationbot/internal/catalog"
)

type adminSession struct {
	LocationID       string
	OriginalHash     string
	Draft            catalog.CardRevision
	Field            string
	SectionIndex     int
	Backup           adminbackup.Archive
	HasBackup        bool
	AwaitingBackup   bool
	StatsMetric      string
	StatsDays        int
	HistoryLocation  string
	HistoryPage      int
	RestoreIndex     int
	RestoreHash      string
	CreatingLocation bool
	CleanupKind      string
	CleanupTrash     bool
	CleanupPage      int
	CleanupSelected  map[int64]bool
	CleanupAction    string
}
type adminRevisionEvent struct {
	At         time.Time            `json:"at"`
	Actor      int64                `json:"actor"`
	LocationID string               `json:"location_id"`
	Before     catalog.CardRevision `json:"before"`
	After      catalog.CardRevision `json:"after"`
	Operation  string               `json:"operation"`
}

func (b *Bot) isAdmin(userID int64) bool { return userID > 0 && b.adminIDs[userID] }
func (b *Bot) adminSession(userID int64) *adminSession {
	b.adminMu.Lock()
	defer b.adminMu.Unlock()
	if b.adminStates == nil {
		b.adminStates = map[int64]*adminSession{}
	}
	if b.adminStates[userID] == nil {
		b.adminStates[userID] = &adminSession{}
	}
	return b.adminStates[userID]
}
func (b *Bot) adminHomeMarkup(mapURL string, userID int64) any {
	return startMenuMarkup(mapURL, b.homeLeadAction(userID), b.isAdmin(userID))
}

func (b *Bot) showAdminMenu(ctx context.Context, chatID int64) error {
	msg := "⚙️ <b>Панель управления</b>\n\nЗдесь можно редактировать и добавлять локации, управлять заявками, просматривать аналитику и посетителей без заявки, работать с резервными копиями и настройками."
	return b.Send(ctx, chatID, msg, markup(
		[]Button{callbackButton("📊 Аналитика", "adm:stats:views:30"), callbackButton("📍 Локации", "adm:locations:0")},
		[]Button{callbackButton("📨 Заявки", "adm:leads"), callbackButton("👥 Посетители", "adm:visitors")},
		[]Button{callbackButton("🕘 История", "adm:history")},
		[]Button{callbackButton("📦 Бэкапы", "adm:backup"), callbackButton("🛠 Система", "adm:tools")},
		[]Button{menuButton()},
	))
}
func (b *Bot) showAdminTools(ctx context.Context, chatID int64) error {
	return b.Send(ctx, chatID, "🛠 <b>Система</b>\n\nЗдесь находятся служебные действия. Они доступны только администраторам.", markup(
		[]Button{callbackButton("☁️ Синхронизация", "adm:sync"), callbackButton("🔄 Перезагрузить каталог", "adm:reload")},
		[]Button{callbackButton("📍 Состояние каталога", "adm:status"), callbackButton("📦 Бэкапы", "adm:backup")},
		[]Button{callbackButton("⬅️ Админ-панель", "adm:menu")},
	))
}
func (b *Bot) adminLeadCount() int {
	if b.leads == nil {
		return 0
	}
	return len(b.leads.visibleSubmitted())
}
func (b *Bot) adminLeadDates() map[string][]time.Time {
	result := map[string][]time.Time{}
	if b.leads == nil {
		return result
	}
	for _, app := range b.leads.visibleSubmitted() {
		for _, loc := range app.locations() {
			at := app.CreatedAt
			for _, u := range app.Updates {
				for _, added := range u.Locations {
					if added.ID == loc.ID {
						at = u.CreatedAt
					}
				}
			}
			if loc.ID != "" {
				result[loc.ID] = append(result[loc.ID], at)
			}
		}
	}
	return result
}
func (b *Bot) adminCurrentFavorites() map[string]int {
	result := map[string]int{}
	if b.favorites == nil {
		return result
	}
	b.favorites.mu.RLock()
	defer b.favorites.mu.RUnlock()
	for userID, ids := range b.favorites.byUser {
		n, err := strconv.ParseInt(userID, 10, 64)
		if err != nil || b.isAdmin(n) {
			continue
		}
		for _, id := range ids {
			result[id]++
		}
	}
	return result
}
func (b *Bot) recordLocationAnalytics(userID int64, id, kind string) {
	if b.metrics == nil || b.isAdmin(userID) {
		return
	}
	if _, ok := b.store.Find(id); !ok {
		return
	}
	if err := b.metrics.Record(userID, id, kind, time.Now().UTC()); err != nil {
		log.Printf("analytics event: %v", err)
	}
}
func (b *Bot) showAdminStats(ctx context.Context, chatID int64, metric string, days int) error {
	switch metric {
	case "views", "unique", "added", "saved", "leads":
	default:
		metric = "views"
	}
	if days != 7 && days != 30 && days != 0 {
		days = 30
	}
	if metric == "saved" {
		days = 0
	}
	if chatID > 0 {
		session := b.adminSession(chatID)
		session.StatsMetric = metric
		session.StatsDays = days
	}
	ids := []string{}
	names := map[string]string{}
	for _, l := range b.store.List() {
		ids = append(ids, l.ID)
		names[l.ID] = publicText(l.Title)
	}
	if b.metrics == nil {
		return b.Send(ctx, chatID, "Аналитика временно недоступна.", nil)
	}
	scores := b.metrics.Rankings(time.Now().UTC(), days, metric, ids, b.adminCurrentFavorites(), b.adminLeadDates())
	definitions := map[string]struct{ label, description string }{
		"views":  {"👁 Просмотры", "Сколько раз клиенты открыли карточку за выбранный период. Повторные открытия одним клиентом в течение 20 секунд считаются одним просмотром."},
		"unique": {"👤 Уникальные просмотры", "Сколько разных клиентов открыли карточку за выбранный период. Каждый клиент учитывается один раз для каждой локации."},
		"added":  {"⭐ Добавления в избранное", "Сколько раз за выбранный период клиенты добавили локацию в избранное. Если клиент позже убрал её, добавление остаётся в статистике."},
		"saved":  {"⭐ Сейчас в избранном", "У скольких клиентов локация сохранена сейчас. Учитываются и сохранения, сделанные до запуска аналитики, если локация всё ещё в избранном."},
		"leads":  {"🎯 Заявки", "В скольких заявках выбрали локацию за указанный период. Учитываются и локации из дополнений — по дате их добавления."},
	}
	selected := definitions[metric]
	period := "За всё время"
	if days > 0 {
		period = fmt.Sprintf("За %d дней", days)
	}
	if metric == "saved" {
		period = "На текущий момент"
	}
	var text strings.Builder
	text.WriteString("📊 <b>ТОП-10 локаций</b>\n" + selected.label + " · " + period + "\n" + selected.description + "\n\n")
	rows := [][]Button{}
	for index, r := range scores {
		n := r.Views
		switch metric {
		case "unique":
			n = r.UniqueViews
		case "added":
			n = r.Additions
		case "saved":
			n = r.CurrentFavorites
		case "leads":
			n = r.Leads
		}
		text.WriteString(fmt.Sprintf("%d. <b>%s</b> — %d\n", index+1, html.EscapeString(names[r.LocationID]), n))
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("%d. %s · %d", index+1, limitRunes(names[r.LocationID], 33), n), "adm:insight:"+r.LocationID+":"+strconv.Itoa(days))})
	}
	rows = append(rows, []Button{callbackButton("👁 Просмотры", "adm:stats:views:"+strconv.Itoa(days)), callbackButton("⭐ Добавления", "adm:stats:added:"+strconv.Itoa(days))})
	rows = append(rows, []Button{callbackButton("👤 Уникальные", "adm:stats:unique:"+strconv.Itoa(days)), callbackButton("🎯 Заявки", "adm:stats:leads:"+strconv.Itoa(days))})
	rows = append(rows, []Button{callbackButton("⭐ Сейчас в избранном", "adm:stats:saved:0")})
	if metric != "saved" {
		rows = append(rows, []Button{callbackButton("7 дней", "adm:stats:"+metric+":7"), callbackButton("30 дней", "adm:stats:"+metric+":30"), callbackButton("Всё время", "adm:stats:"+metric+":0")})
	}
	rows = append(rows, []Button{callbackButton("⬅️ Админ-панель", "adm:menu")})
	return b.Send(ctx, chatID, text.String(), markup(rows...))
}
func (b *Bot) showLocationInsights(ctx context.Context, chatID int64, locationID string, days int) error {
	loc, ok := b.store.Find(locationID)
	if !ok {
		return b.Send(ctx, chatID, "Локация не найдена.", nil)
	}
	if days != 7 && days != 30 && days != 0 {
		days = 30
	}
	if b.metrics == nil {
		return b.Send(ctx, chatID, "Аналитика временно недоступна.", nil)
	}
	ranks := b.metrics.Rankings(time.Now().UTC(), days, "views", []string{locationID}, b.adminCurrentFavorites(), b.adminLeadDates())
	if len(ranks) != 1 {
		return b.Send(ctx, chatID, "Статистика пока недоступна.", nil)
	}
	r := ranks[0]
	period := "за всё время"
	if days > 0 {
		period = fmt.Sprintf("за %d дней", days)
	}
	description := fmt.Sprintf("📍 <b>%s</b> · <code>%s</code>\n\n📊 Показатели %s\n👁 Открытий: <b>%d</b>\n👤 Уникальных: <b>%d</b>\n⭐ Добавлений в избранное: <b>%d</b>\n🎯 Заявок: <b>%d</b>\n\n⭐ Сейчас в избранном у клиентов: <b>%d</b>", html.EscapeString(publicText(loc.Title)), html.EscapeString(loc.ID), period, r.Views, r.UniqueViews, r.Additions, r.Leads, r.CurrentFavorites)
	session := b.adminSession(chatID)
	metric := session.StatsMetric
	if metric == "" {
		metric = "views"
	}
	return b.Send(ctx, chatID, description, markup(
		[]Button{callbackButton("7 дней", "adm:insight:"+locationID+":7"), callbackButton("30 дней", "adm:insight:"+locationID+":30"), callbackButton("Всё время", "adm:insight:"+locationID+":0")},
		[]Button{callbackButton("✏️ Редактор", "adm:edit:"+locationID), callbackButton("🕘 История", "adm:histloc:"+locationID)},
		[]Button{callbackButton("⬅️ К рейтингу", "adm:stats:"+metric+":"+strconv.Itoa(session.StatsDays))},
	))
}
func (b *Bot) showAdminLocations(ctx context.Context, chatID int64, page int) error {
	locations := sortedStations(b.store.List())
	pages := (len(locations) + 9) / 10
	if pages == 0 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	from := page * 10
	until := from + 10
	if until > len(locations) {
		until = len(locations)
	}
	rows := [][]Button{}
	for i := from; i < until; i += 2 {
		row := []Button{callbackButton(buttonLabel(locations[i].Title), "adm:edit:"+locations[i].ID)}
		if i+1 < until {
			row = append(row, callbackButton(buttonLabel(locations[i+1].Title), "adm:edit:"+locations[i+1].ID))
		}
		rows = append(rows, row)
	}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️", fmt.Sprintf("adm:locations:%d", page-1)))
	}
	if page+1 < pages {
		nav = append(nav, callbackButton("▶️", fmt.Sprintf("adm:locations:%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{callbackButton("🔎 Найти локацию", "adm:locsearch"), callbackButton("➕ Добавить локацию", "adm:new")})
	rows = append(rows, []Button{callbackButton("⬅️ Админ-панель", "adm:menu")})
	return b.Send(ctx, chatID, fmt.Sprintf("📍 <b>Редактор локаций</b>\nСтраница %d из %d. Выберите карточку для редактирования:", page+1, pages), markup(rows...))
}

// A new location is an unpublished draft until the administrator confirms it.
func (b *Bot) startAdminNewLocation(ctx context.Context, chatID, userID int64) error {
	id, err := catalog.NewAdminLocationID()
	if err != nil {
		return err
	}
	session := b.adminSession(userID)
	*session = adminSession{
		LocationID: id, CreatingLocation: true, Field: "new_title",
		Draft: catalog.CardRevision{Sections: []catalog.CardSection{}},
	}
	return b.Send(ctx, chatID, "➕ <b>Новая локация</b>\n\nВведите название новой локации обычным сообщением. Затем сможете добавить ориентир, статус, описание, разделы карточки и опубликовать её.\n\nДо публикации локация не видна пользователям.", markup(
		[]Button{callbackButton("❌ Отмена", "adm:locations:0")},
	))
}

// Every edit displays the currently saved draft value before accepting input.
func adminCurrentValue(session *adminSession, field string) (string, string, bool) {
	switch field {
	case "title":
		return "Название", session.Draft.Title, true
	case "line_status":
		return "Статус", session.Draft.LineStatus, true
	case "landmarks":
		return "Ориентир", session.Draft.Landmarks, true
	case "map_url":
		return "Ссылка на карту", session.Draft.MapURL, true
	case "description":
		return "Краткое описание", session.Draft.Description, true
	case "category":
		return "Категория", session.Draft.Category, true
	case "section_title", "section_body":
		if session.SectionIndex < 0 || session.SectionIndex >= len(session.Draft.Sections) {
			return "", "", false
		}
		section := session.Draft.Sections[session.SectionIndex]
		if field == "section_title" {
			return "Название раздела", section.Title, true
		}
		return "Описание раздела", section.Body, true
	}
	return "", "", false
}
func (b *Bot) showAdminFieldPrompt(ctx context.Context, chatID int64, session *adminSession, field string) error {
	label, value, ok := adminCurrentValue(session, field)
	if !ok {
		return b.showAdminEditor(ctx, chatID, session)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = "Не заполнено"
	}
	// Prevent Telegram's 4096 character message limit on long descriptions.
	current := html.EscapeString(limitRunes(value, 2000))
	back := "adm:editor"
	if strings.HasPrefix(field, "section_") {
		back = "adm:sections"
	}
	return b.Send(ctx, chatID, "✏️ <b>"+html.EscapeString(label)+"</b>\n\n<b>Текущее значение:</b>\n"+current+"\n\nОтправьте новое значение обычным сообщением. Изменение останется черновиком до публикации.", markup(
		[]Button{callbackButton("❌ Отмена", back)},
	))
}

func (b *Bot) startAdminEditor(ctx context.Context, chatID, userID int64, id string) error {
	loc, ok := b.store.Find(id)
	if !ok {
		return b.Send(ctx, chatID, "Локация не найдена.", nil)
	}
	session := b.adminSession(userID)
	session.LocationID = id
	session.CreatingLocation = false
	session.Draft = catalog.Revision(loc)
	session.OriginalHash = catalog.RevisionHash(session.Draft)
	session.Field = ""
	return b.showAdminEditor(ctx, chatID, session)
}
func (b *Bot) showAdminEditor(ctx context.Context, chatID int64, s *adminSession) error {
	name := html.EscapeString(s.Draft.Title)
	rows := [][]Button{
		{callbackButton("✏️ Название", "adm:field:title"), callbackButton("📌 Статус", "adm:field:line_status")},
		{callbackButton("📍 Ориентир", "adm:field:landmarks"), callbackButton("🗺 Ссылка на карту", "adm:field:map_url")},
		{callbackButton("📝 Краткое описание", "adm:field:description"), callbackButton("📌 Категория", "adm:field:category")},
		{callbackButton("🧩 Разделы карточки", "adm:sections")},
	}
	header := "✏️ <b>Редактор карточки</b>"
	publish := "✅ Опубликовать"
	if s.CreatingLocation {
		header = "➕ <b>Новая локация · черновик</b>"
		publish = "✅ Добавить локацию"
	} else {
		rows = append(rows, []Button{callbackButton("📊 Показатели", "adm:insight:"+s.LocationID+":30"), callbackButton("🕘 История", "adm:histloc:"+s.LocationID)})
	}
	rows = append(rows,
		[]Button{callbackButton("👁 Предпросмотр", "adm:preview"), callbackButton(publish, "adm:publish")},
		[]Button{callbackButton("❌ Отменить", "adm:locations:0"), callbackButton("⬅️ Админ-панель", "adm:menu")},
	)
	return b.Send(ctx, chatID, fmt.Sprintf("%s\n\n<b>%s</b> · ID <code>%s</code>\nРазделов: %d\n\nИзменения не увидят пользователи до публикации.", header, name, html.EscapeString(s.LocationID), len(s.Draft.Sections)), markup(rows...))
}
func (b *Bot) showAdminSections(ctx context.Context, chatID int64, s *adminSession) error {
	rows := [][]Button{}
	for i, section := range s.Draft.Sections {
		mark := "✅"
		if !section.Enabled {
			mark = "🚫"
		}
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("%s %d. %s", mark, i+1, limitRunes(section.Title, 45)), fmt.Sprintf("adm:section:%d", i))})
	}
	rows = append(rows, []Button{callbackButton("➕ Добавить раздел", "adm:add")})
	rows = append(rows, []Button{callbackButton("⬅️ К карточке", "adm:editor")})
	return b.Send(ctx, chatID, "🧩 <b>Разделы карточки</b>\n\nНажмите нужный раздел для изменения текста, названия, порядка или видимости.", markup(rows...))
}
func (b *Bot) showAdminSection(ctx context.Context, chatID int64, s *adminSession, i int) error {
	if i < 0 || i >= len(s.Draft.Sections) {
		return b.showAdminSections(ctx, chatID, s)
	}
	section := s.Draft.Sections[i]
	label := "Показывается"
	if !section.Enabled {
		label = "Скрыт"
	}
	rows := [][]Button{
		{callbackButton("✏️ Заголовок", fmt.Sprintf("adm:sectiontitle:%d", i)), callbackButton("📝 Описание", fmt.Sprintf("adm:sectionbody:%d", i))},
		{callbackButton("◀️ Выше", fmt.Sprintf("adm:up:%d", i)), callbackButton("Ниже ▶️", fmt.Sprintf("adm:down:%d", i))},
		{callbackButton("👁 Скрыть / показать", fmt.Sprintf("adm:toggle:%d", i)), callbackButton("🗑 Удалить", fmt.Sprintf("adm:remove:%d", i))},
		{callbackButton("⬅️ К разделам", "adm:sections")},
	}
	return b.Send(ctx, chatID, fmt.Sprintf("🧩 <b>%s</b>\nСостояние: %s\n\n%s", html.EscapeString(section.Title), label, html.EscapeString(limitRunes(section.Body, 650))), markup(rows...))
}
func (b *Bot) renderAdminPreview(ctx context.Context, chatID int64, s *adminSession) error {
	loc, found := b.store.Find(s.LocationID)
	if !found && !s.CreatingLocation {
		return errors.New("location missing")
	}
	if s.CreatingLocation {
		loc = catalog.Location{ID: s.LocationID}
	}
	loc.Title = s.Draft.Title
	loc.LineStatus = s.Draft.LineStatus
	loc.Landmarks = s.Draft.Landmarks
	loc.Description = s.Draft.Description
	loc.Category = s.Draft.Category
	loc.MapURL = s.Draft.MapURL
	loc.Sections = append([]catalog.CardSection{}, s.Draft.Sections...)
	return b.Send(ctx, chatID, "👁 <b>Предпросмотр карточки</b>\n\n"+cardPreviewHTML(loc, b.mapURL), markup([]Button{callbackButton("⬅️ Редактировать", "adm:editor"), callbackButton("✅ Опубликовать", "adm:publish")}))
}
func cardPreviewHTML(l catalog.Location, mapURL string) string {
	// Rich preview uses simple HTML markup that standard Telegram clients can
	// render, while production Rich Messages keep their native details.
	var buf strings.Builder
	buf.WriteString("📍 <b>" + html.EscapeString(l.Title) + "</b>\n")
	buf.WriteString("<i>" + html.EscapeString(l.LineStatus) + "</i>\nОриентир: " + html.EscapeString(l.Landmarks) + "\n")
	for _, s := range l.Sections {
		if s.Enabled {
			buf.WriteString("\n<b>" + html.EscapeString(s.Title) + "</b>\n" + html.EscapeString(s.Body) + "\n")
		}
	}
	return buf.String()
}
func (b *Bot) adminHistoryPath() string {
	return filepath.Join(b.store.DataDir(), "admin_history.json")
}
func (b *Bot) adminHistory() ([]adminRevisionEvent, error) {
	data, err := os.ReadFile(b.adminHistoryPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []adminRevisionEvent
	if err = json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
func (b *Bot) recordAdminHistory(entry adminRevisionEvent) error {
	entries, err := b.adminHistory()
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return atomicAdminFile(b.adminHistoryPath(), data)
}
func atomicAdminFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".admin-history-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (b *Bot) publishNewAdminLocation(ctx context.Context, chatID, userID int64, s *adminSession) error {
	if err := s.Draft.Validate(); err != nil {
		return b.Send(ctx, chatID, "Не удалось опубликовать: "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К редактору", "adm:editor")}))
	}
	loc := catalog.Location{
		ID: s.LocationID, Title: s.Draft.Title, Category: s.Draft.Category,
		Description: s.Draft.Description, Rating: s.Draft.Rating,
		LineStatus: s.Draft.LineStatus, Landmarks: s.Draft.Landmarks, MapURL: s.Draft.MapURL,
		Latitude: s.Draft.Latitude, Longitude: s.Draft.Longitude,
		Sections: append([]catalog.CardSection{}, s.Draft.Sections...),
	}
	if err := b.store.CreateAdminLocation(loc); err != nil {
		return b.Send(ctx, chatID, "Не удалось добавить локацию: "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К редактору", "adm:editor")}))
	}
	s.CreatingLocation = false
	s.OriginalHash = catalog.RevisionHash(s.Draft)
	entry := adminRevisionEvent{At: time.Now().UTC(), Actor: userID, LocationID: s.LocationID, After: s.Draft, Operation: "create"}
	if err := b.recordAdminHistory(entry); err != nil {
		log.Printf("New admin location history failure: %v", err)
		return b.Send(ctx, chatID, "✅ Локация добавлена, но запись истории не сохранилась. Сделайте резервную копию.", markup([]Button{callbackButton("⬅️ Админ-панель", "adm:menu")}))
	}
	return b.Send(ctx, chatID, "✅ <b>Локация добавлена в каталог!</b>\n\n"+html.EscapeString(s.Draft.Title)+"\n\nОна доступна пользователям и сохранится после перезапуска бота.", markup(
		[]Button{callbackButton("✏️ Редактировать", "adm:editor"), callbackButton("📍 К списку", "adm:locations:0")},
		[]Button{callbackButton("⬅️ Админ-панель", "adm:menu")},
	))
}

func (b *Bot) publishAdminDraft(ctx context.Context, chatID, userID int64, s *adminSession) error {
	if s.CreatingLocation {
		return b.publishNewAdminLocation(ctx, chatID, userID, s)
	}
	loc, ok := b.store.Find(s.LocationID)
	if !ok {
		return errors.New("missing location")
	}
	before := catalog.Revision(loc)
	if catalog.RevisionHash(before) == catalog.RevisionHash(s.Draft) {
		return b.Send(ctx, chatID, "Нет новых изменений для публикации.", markup([]Button{callbackButton("⬅️ К редактору", "adm:editor")}))
	}
	if err := b.store.AdminEdit(s.LocationID, s.OriginalHash, s.Draft); err != nil {
		return b.Send(ctx, chatID, "Не удалось опубликовать: "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К редактору", "adm:editor")}))
	}
	entry := adminRevisionEvent{At: time.Now().UTC(), Actor: userID, LocationID: s.LocationID, Before: before, After: s.Draft, Operation: "publish"}
	if err := b.recordAdminHistory(entry); err != nil {
		log.Printf("Admin history write failed: %v", err)
		return b.Send(ctx, chatID, "Карточка опубликована, но журнал изменений не сохранился. Рекомендуется сделать резервную копию.", markup([]Button{callbackButton("⬅️ Админка", "adm:menu")}))
	}
	s.OriginalHash = catalog.RevisionHash(s.Draft)
	return b.Send(ctx, chatID, "✅ <b>Карточка опубликована!</b>\n\nВсе новые открытия карточки будут показывать обновлённые данные.", markup([]Button{callbackButton("✏️ Продолжить редактирование", "adm:editor")}, []Button{callbackButton("⬅️ Админ-панель", "adm:menu")}))
}

// A paged, location-filtered revision log. Every row opens an actual version.
func (b *Bot) showAdminHistory(ctx context.Context, chatID int64) error {
	session := b.adminSession(chatID)
	session.HistoryLocation = ""
	return b.showAdminHistoryPage(ctx, chatID, 0)
}
func (b *Bot) showAdminHistoryPage(ctx context.Context, chatID int64, page int) error {
	session := b.adminSession(chatID)
	records, err := b.adminHistory()
	if err != nil {
		return err
	}
	indexes := []int{}
	for i := len(records) - 1; i >= 0; i-- {
		if session.HistoryLocation == "" || records[i].LocationID == session.HistoryLocation {
			indexes = append(indexes, i)
		}
	}
	pages := (len(indexes) + 7) / 8
	if pages < 1 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	session.HistoryPage = page
	var msg strings.Builder
	msg.WriteString("🕘 <b>История редакций карточек</b>")
	if session.HistoryLocation != "" {
		msg.WriteString("\nЛокация: <code>" + html.EscapeString(session.HistoryLocation) + "</code>")
	}
	msg.WriteString(fmt.Sprintf("\nЗаписей: %d · Страница %d\n\n", len(indexes), page+1))
	rows := [][]Button{}
	for _, index := range indexes[page*8 : min((page+1)*8, len(indexes))] {
		e := records[index]
		msg.WriteString(fmt.Sprintf("• %s · %s · %s\n", e.At.UTC().Format("02.01 15:04"), html.EscapeString(e.LocationID), html.EscapeString(e.Operation)))
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("🕘 %s · %s", e.At.Format("02.01 15:04"), e.LocationID), fmt.Sprintf("adm:histview:%d", index))})
	}
	if len(indexes) == 0 {
		msg.WriteString("Изменений пока нет.")
	}
	paging := []Button{}
	if page > 0 {
		paging = append(paging, callbackButton("◀️ Ранее", fmt.Sprintf("adm:histpage:%d", page-1)))
	}
	if (page+1)*8 < len(indexes) {
		paging = append(paging, callbackButton("Позже ▶️", fmt.Sprintf("adm:histpage:%d", page+1)))
	}
	if len(paging) > 0 {
		rows = append(rows, paging)
	}
	if session.HistoryLocation != "" {
		rows = append(rows, []Button{callbackButton("📍 К карточке", "adm:edit:"+session.HistoryLocation), callbackButton("🕘 Вся история", "adm:history")})
	}
	rows = append(rows, []Button{callbackButton("⬅️ Админ-панель", "adm:menu")})
	return b.Send(ctx, chatID, msg.String(), markup(rows...))
}
func briefDiff(before, after catalog.CardRevision) string {
	changes := []string{}
	diff := func(label, a, b string) {
		if a != b {
			changes = append(changes, html.EscapeString(label)+": <i>"+html.EscapeString(limitRunes(a, 75))+"</i> → <b>"+html.EscapeString(limitRunes(b, 75))+"</b>")
		}
	}
	diff("Название", before.Title, after.Title)
	diff("Категория", before.Category, after.Category)
	diff("Статус", before.LineStatus, after.LineStatus)
	diff("Ориентир", before.Landmarks, after.Landmarks)
	diff("Ссылка карты", before.MapURL, after.MapURL)
	diff("Описание", before.Description, after.Description)
	old := map[string]catalog.CardSection{}
	new := map[string]catalog.CardSection{}
	for _, v := range before.Sections {
		old[v.ID] = v
	}
	for _, v := range after.Sections {
		new[v.ID] = v
	}
	for _, v := range before.Sections {
		n, ok := new[v.ID]
		if !ok {
			changes = append(changes, "🗑 Удалён: "+html.EscapeString(v.Title))
			continue
		}
		if v.Title != n.Title || v.Body != n.Body || v.Enabled != n.Enabled {
			changes = append(changes, "✏️ Раздел: "+html.EscapeString(limitRunes(n.Title, 65)))
		}
	}
	for _, v := range after.Sections {
		if _, ok := old[v.ID]; !ok {
			changes = append(changes, "➕ Добавлен: "+html.EscapeString(v.Title))
		}
	}
	beforeOrder := []string{}
	afterOrder := []string{}
	for _, v := range before.Sections {
		beforeOrder = append(beforeOrder, v.ID)
	}
	for _, v := range after.Sections {
		afterOrder = append(afterOrder, v.ID)
	}
	if strings.Join(beforeOrder, ",") != strings.Join(afterOrder, ",") {
		changes = append(changes, "↕️ Изменён порядок разделов")
	}
	if len(changes) == 0 {
		return "Изменений в видимых полях нет."
	}
	return strings.Join(changes, "\n")
}
func (b *Bot) showAdminHistoryEntry(ctx context.Context, chatID int64, index int) error {
	records, err := b.adminHistory()
	if err != nil {
		return err
	}
	if index < 0 || index >= len(records) {
		return b.showAdminHistoryPage(ctx, chatID, b.adminSession(chatID).HistoryPage)
	}
	item := records[index]
	summary := fmt.Sprintf("🕘 <b>Редакция #%d</b> · <code>%s</code>\n%s UTC · Администратор: <code>%d</code>\nДействие: %s\n\n<b>Что изменилось</b>\n%s", index+1, html.EscapeString(item.LocationID), item.At.UTC().Format("02.01.2006 15:04"), item.Actor, html.EscapeString(item.Operation), briefDiff(item.Before, item.After))
	rows := [][]Button{}
	// A creation entry has no previous published card to roll back to.
	if item.Operation != "create" {
		rows = append(rows, []Button{callbackButton("↩️ Восстановить версию до правки", fmt.Sprintf("adm:histrestore:%d", index))})
	}
	rows = append(rows, []Button{callbackButton("⬅️ К истории", fmt.Sprintf("adm:histpage:%d", b.adminSession(chatID).HistoryPage))})
	return b.Send(ctx, chatID, summary, markup(rows...))
}
func (b *Bot) confirmAdminHistoryRestore(ctx context.Context, chatID int64, index int) error {
	records, err := b.adminHistory()
	if err != nil {
		return err
	}
	if index < 0 || index >= len(records) {
		return b.showAdminHistoryPage(ctx, chatID, 0)
	}
	if records[index].Operation == "create" {
		return b.Send(ctx, chatID, "Создание новой локации нельзя отменить как обычное изменение раздела.", markup([]Button{callbackButton("⬅️ К истории", "adm:history")}))
	}
	loc, ok := b.store.Find(records[index].LocationID)
	if !ok {
		return b.Send(ctx, chatID, "Локация больше не найдена.", nil)
	}
	session := b.adminSession(chatID)
	session.RestoreIndex = index
	session.RestoreHash = catalog.RevisionHash(catalog.Revision(loc))
	return b.Send(ctx, chatID, "⚠️ <b>Подтвердите откат</b>\n\nБудет восстановлена версия карточки <b>до выбранной правки</b>. Текущая версия останется в истории.", markup(
		[]Button{callbackButton("✅ Восстановить", fmt.Sprintf("adm:histapply:%d", index))},
		[]Button{callbackButton("❌ Отмена", fmt.Sprintf("adm:histview:%d", index))},
	))
}
func (b *Bot) applyAdminHistoryRestore(ctx context.Context, chatID, userID int64, index int) error {
	session := b.adminSession(userID)
	if session.RestoreIndex != index || session.RestoreHash == "" {
		return b.Send(ctx, chatID, "Подтверждение устарело. Выберите версию заново.", nil)
	}
	confirmedHash := session.RestoreHash
	session.RestoreHash, session.RestoreIndex = "", -1
	records, err := b.adminHistory()
	if err != nil {
		return err
	}
	if index < 0 || index >= len(records) {
		return b.showAdminHistoryPage(ctx, chatID, 0)
	}
	item := records[index]
	if item.Operation == "create" {
		return b.Send(ctx, chatID, "Нельзя восстановить пустую версию до создания карточки.", nil)
	}
	current, ok := b.store.Find(item.LocationID)
	if !ok {
		return errors.New("location missing")
	}
	expected := catalog.RevisionHash(catalog.Revision(current))
	// The confirmation stored the hash so edits by another admin cannot be lost.
	if expected != confirmedHash {
		return b.Send(ctx, chatID, "Карточка изменилась после подтверждения. Повторите выбор версии.", nil)
	}
	if err := b.store.AdminEdit(item.LocationID, expected, item.Before); err != nil {
		return err
	}
	if err := b.recordAdminHistory(adminRevisionEvent{At: time.Now().UTC(), Actor: userID, LocationID: item.LocationID, Before: catalog.Revision(current), After: item.Before, Operation: "restore"}); err != nil {
		return err
	}
	return b.Send(ctx, chatID, "✅ Редакция восстановлена. Предыдущее состояние записано в историю.", markup([]Button{callbackButton("🕘 История", "adm:history"), callbackButton("✏️ Карточка", "adm:edit:"+item.LocationID)}))
}
func (b *Bot) undoLastAdminChange(ctx context.Context, chatID, userID int64) error {
	records, err := b.adminHistory()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return b.showAdminHistory(ctx, chatID)
	}
	last := records[len(records)-1]
	if last.Operation == "create" {
		return b.Send(ctx, chatID, "Для созданной локации нет предыдущей редакции.", nil)
	}
	current, ok := b.store.Find(last.LocationID)
	if !ok {
		return errors.New("missing location")
	}
	if err := b.store.AdminEdit(last.LocationID, catalog.RevisionHash(last.After), last.Before); err != nil {
		return err
	}
	if err := b.recordAdminHistory(adminRevisionEvent{At: time.Now().UTC(), Actor: userID, LocationID: last.LocationID, Before: catalog.Revision(current), After: last.Before, Operation: "undo"}); err != nil {
		return err
	}
	return b.Send(ctx, chatID, "✅ Предыдущая редакция карточки восстановлена.", markup([]Button{callbackButton("🕘 История", "adm:history")}))
}
func (b *Bot) showBackupMenu(ctx context.Context, chatID int64) error {
	list, err := b.autoBackupList()
	if err != nil {
		return err
	}
	last := "ещё нет"
	if len(list) > 0 {
		last = strings.TrimSuffix(strings.TrimPrefix(list[len(list)-1], "auto-"), ".zip")
	}
	info := fmt.Sprintf("📦 <b>Резервные копии</b>\n\nЕжедневный бэкап: 03:00 UTC\nХраним последние %d архивов на сервере.\nПоследняя копия: <b>%s</b>\n\nАрхив содержит заявки, карточки и избранное. Отправляйте ZIP только администраторам.", automaticBackupKeep, last)
	return b.Send(ctx, chatID, info, markup(
		[]Button{callbackButton("📥 Скачать свежий ZIP", "adm:export")},
		[]Button{callbackButton("📥 Скачать последний авто ZIP", "adm:exportauto")},
		[]Button{callbackButton("📤 Загрузить бэкап", "adm:import")},
		[]Button{callbackButton("⬅️ Админ-панель", "adm:menu")},
	))
}
func (b *Bot) sendAdminBackup(ctx context.Context, chatID int64) error {
	b.backupMu.Lock()
	defer b.backupMu.Unlock()
	data, err := b.snapshotBackup()
	if err != nil {
		return err
	}
	filename := "backup-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	return b.sendAdminDocument(ctx, chatID, data, filename)
}
func (b *Bot) sendLatestAutoBackup(ctx context.Context, chatID int64) error {
	b.backupMu.Lock()
	defer b.backupMu.Unlock()
	files, err := b.autoBackupList()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return b.Send(ctx, chatID, "Автоматических копий ещё нет.", markup([]Button{callbackButton("⬅️ Бэкапы", "adm:backup")}))
	}
	filename := files[len(files)-1]
	data, err := os.ReadFile(filepath.Join(b.autoBackupDirectory(), filename))
	if err != nil {
		return err
	}
	return b.sendAdminDocument(ctx, chatID, data, filename)
}
func (b *Bot) sendAdminDocument(ctx context.Context, chatID int64, data []byte, filename string) error {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	file, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.telegram.org/bot"+b.token+"/sendDocument", &buffer)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result apiResult
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("Telegram sendDocument: %s", result.Description)
	}
	return nil
}
func (b *Bot) receiveAdminBackup(ctx context.Context, chatID, userID int64, fileID string, size int64) error {
	if fileID == "" || size > adminbackup.MaxZIP || size < 0 {
		return b.Send(ctx, chatID, "Архив слишком большой или не распознан. Максимум 15 МБ.", nil)
	}
	raw, err := b.call(ctx, "getFile", map[string]any{"file_id": fileID})
	if err != nil {
		return err
	}
	var response struct {
		FilePath string `json:"file_path"`
		FileSize int64  `json:"file_size"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return err
	}
	path := response.FilePath
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || path == "" {
		return errors.New("invalid Telegram file path")
	}
	if response.FileSize > adminbackup.MaxZIP {
		return errors.New("backup too large")
	}
	safePath := strings.Split(path, "/")
	for _, part := range safePath {
		if part == "" || part == "." || part == ".." {
			return errors.New("invalid Telegram file path")
		}
	}
	downloadURL := "https://api.telegram.org/file/bot" + url.PathEscape(b.token) + "/" + strings.Join(safePath, "/")
	request, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := b.http.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram file download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, adminbackup.MaxZIP+1))
	if err != nil {
		return err
	}
	if len(data) > adminbackup.MaxZIP {
		return errors.New("backup download exceeds limit")
	}
	archive, err := adminbackup.Examine(data)
	if err != nil {
		return b.Send(ctx, chatID, "❌ Архив не прошёл проверку: "+html.EscapeString(err.Error()), nil)
	}
	session := b.adminSession(userID)
	session.Backup = archive
	session.HasBackup = true
	session.AwaitingBackup = false
	inspection := archive.Inspect
	return b.Send(ctx, chatID, fmt.Sprintf("📦 <b>Проверка резервной копии</b>\n\nДата: %s UTC\nЛокаций: %d\nЗаявок: %d\nСохранений: %d\n\nВыберите режим. Перед восстановлением бот создаст защитный бэкап.", inspection.CreatedAt.Format("02.01.2006 15:04"), inspection.Locations, inspection.Leads, inspection.Favorites), markup(
		[]Button{callbackButton("📍 Только карточки", "adm:restore:cards")},
		[]Button{callbackButton("⚠️ Полное восстановление", "adm:restore:full")},
		[]Button{callbackButton("❌ Отмена", "adm:backup")},
	))
}
func (b *Bot) restoreAdminBackup(ctx context.Context, chatID, userID int64, mode string) error {
	session := b.adminSession(userID)
	if !session.HasBackup {
		return b.showBackupMenu(ctx, chatID)
	}
	b.backupMu.Lock()
	// Scoped helper to ensure all locks are released before sending a Telegram message.
	protective, err := func() (string, error) {
		if mode == "full" {
			if b.leads != nil {
				b.leads.mu.Lock()
				defer b.leads.mu.Unlock()
			}
			if b.favorites != nil {
				b.favorites.mu.Lock()
				defer b.favorites.mu.Unlock()
			}
			if b.visitors != nil {
				b.visitors.mu.Lock()
				defer b.visitors.mu.Unlock()
			}
		}
		var path string
		action := func() error {
			var e error
			path, e = adminbackup.Restore(b.store.DataDir(), session.Backup, mode, b.store.List())
			if e != nil {
				return e
			}
			if e = b.store.ReloadAdminCards(); e != nil {
				return fmt.Errorf("catalog reload (protective backup %s): %w", path, e)
			}
			if mode == "full" {
				fav, e := openFavoriteStore(filepath.Join(b.store.DataDir(), "favorites.json"))
				if e != nil {
					return e
				}
				if b.favorites != nil {
					b.favorites.byUser = fav.byUser
				} else {
					b.favorites = fav
				}
				if e = b.reloadLeadsAfterRestore(); e != nil {
					return e
				}
				if b.visitors != nil {
					v, e := openVisitorStore(filepath.Join(b.store.DataDir(), "visitors.json"))
					if e != nil {
						return e
					}
					b.visitors.users = v.users
				}
			}
			return nil
		}
		if mode == "full" && b.metrics != nil {
			return path, b.metrics.WithExclusiveFile(action)
		}
		return path, action()
	}()
	b.backupMu.Unlock()
	if err != nil {
		return b.Send(ctx, chatID, "❌ Восстановление не удалось: "+html.EscapeString(err.Error()), nil)
	}
	session.HasBackup = false
	return b.Send(ctx, chatID, "✅ <b>Данные восстановлены.</b>\n\nЗащитная копия сохранена на сервере: <code>"+html.EscapeString(filepath.Base(protective))+"</code>", markup([]Button{callbackButton("⬅️ Админ-панель", "adm:menu")}))
}

func (b *Bot) reloadLeadsAfterRestore() error {
	newStore, err := openLeadStore(filepath.Join(b.store.DataDir(), "leads.json"))
	if err != nil {
		return err
	}
	if b.leads != nil {
		b.leads.state = newStore.state
	} else {
		b.leads = newStore
	}
	return nil
}

func (b *Bot) handleAdminCallback(ctx context.Context, chatID int64, from user, data string) error {
	if strings.HasPrefix(data, "adm:visitorfavs:") {
		if !b.isAdmin(from.ID) || chatID != from.ID {
			return nil
		}
		parts := strings.Split(data, ":")
		if len(parts) != 4 {
			return nil
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		page, e := strconv.Atoi(parts[3])
		if err != nil || e != nil || id <= 0 {
			return nil
		}
		return b.showVisitorFavorites(ctx, chatID, id, page)
	}

	if !b.isAdmin(from.ID) || chatID != from.ID {
		return nil
	}
	session := b.adminSession(from.ID)
	if handled, err := b.handleAdminCleanup(ctx, chatID, from.ID, data); handled {
		return err
	}
	parts := strings.Split(data, ":")
	switch {
	case data == "adm:menu":
		session.Field = ""
		return b.showAdminMenu(ctx, chatID)
	case data == "adm:visitors":
		return b.showVisitors(ctx, chatID)
	case strings.HasPrefix(data, "adm:visitors:"):
		p := strings.Split(data, ":")
		if len(p) != 4 {
			return b.showVisitors(ctx, chatID)
		}
		page, err := strconv.Atoi(p[3])
		if err != nil {
			return b.showVisitors(ctx, chatID)
		}
		return b.showVisitorsPage(ctx, chatID, p[2], page)
	case strings.HasPrefix(data, "adm:visitor:"):
		id, err := strconv.ParseInt(strings.TrimPrefix(data, "adm:visitor:"), 10, 64)
		if err != nil || id <= 0 {
			return b.showVisitors(ctx, chatID)
		}
		return b.showVisitor(ctx, chatID, id)
	case data == "adm:backup":
		session.Field = ""
		session.AwaitingBackup = false
		session.HasBackup = false
		return b.showBackupMenu(ctx, chatID)
	case data == "adm:exportauto":
		if err := b.sendLatestAutoBackup(ctx, chatID); err != nil {
			return err
		}
		return b.showBackupMenu(ctx, chatID)
	case data == "adm:export":
		if err := b.sendAdminBackup(ctx, chatID); err != nil {
			return err
		}
		return b.showBackupMenu(ctx, chatID)
	case data == "adm:import":
		session.AwaitingBackup = true
		return b.Send(ctx, chatID, "📤 <b>Загрузить резервную копию</b>\n\nОтправьте ZIP-файл сюда, в личный чат. После проверки архива Вы сможете выбрать тип восстановления.", markup([]Button{callbackButton("⬅️ Отмена", "adm:backup")}))
	case data == "adm:restore:full" || data == "adm:restore:cards":
		if !session.HasBackup {
			return b.showBackupMenu(ctx, chatID)
		}
		mode := strings.TrimPrefix(data, "adm:restore:")
		return b.Send(ctx, chatID, "⚠️ <b>Подтверждение восстановления</b>\n\nВы действительно хотите восстановить данные из архива? Перед операцией будет создан защитный ZIP.", markup(
			[]Button{callbackButton("✅ Да, восстановить", "adm:apply:"+mode)},
			[]Button{callbackButton("❌ Отмена", "adm:backup")},
		))
	case strings.HasPrefix(data, "adm:apply:"):
		return b.restoreAdminBackup(ctx, chatID, from.ID, strings.TrimPrefix(data, "adm:apply:"))
	case data == "adm:history":
		return b.showAdminHistory(ctx, chatID)
	case strings.HasPrefix(data, "adm:histloc:"):
		session.HistoryLocation = strings.TrimPrefix(data, "adm:histloc:")
		if _, ok := b.store.Find(session.HistoryLocation); !ok {
			return b.showAdminHistory(ctx, chatID)
		}
		return b.showAdminHistoryPage(ctx, chatID, 0)
	case strings.HasPrefix(data, "adm:histpage:"):
		page, err := strconv.Atoi(strings.TrimPrefix(data, "adm:histpage:"))
		if err != nil {
			page = 0
		}
		return b.showAdminHistoryPage(ctx, chatID, page)
	case strings.HasPrefix(data, "adm:histview:"):
		n, err := strconv.Atoi(strings.TrimPrefix(data, "adm:histview:"))
		if err != nil {
			return nil
		}
		return b.showAdminHistoryEntry(ctx, chatID, n)
	case strings.HasPrefix(data, "adm:histrestore:"):
		n, err := strconv.Atoi(strings.TrimPrefix(data, "adm:histrestore:"))
		if err != nil {
			return nil
		}
		return b.confirmAdminHistoryRestore(ctx, chatID, n)
	case strings.HasPrefix(data, "adm:histapply:"):
		n, err := strconv.Atoi(strings.TrimPrefix(data, "adm:histapply:"))
		if err != nil {
			return nil
		}
		return b.applyAdminHistoryRestore(ctx, chatID, from.ID, n)
	case data == "adm:undo":
		entries, err := b.adminHistory()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return b.showAdminHistory(ctx, chatID)
		}
		return b.showAdminHistoryEntry(ctx, chatID, len(entries)-1)
	case strings.HasPrefix(data, "adm:insight:") && len(parts) == 4:
		days, err := strconv.Atoi(parts[3])
		if err != nil {
			days = 30
		}
		return b.showLocationInsights(ctx, chatID, parts[2], days)
	case data == "adm:leads":
		return b.showAdminLeads(ctx, chatID)
	case strings.HasPrefix(data, "adm:leadtake:"):
		number, err := strconv.ParseInt(strings.TrimPrefix(data, "adm:leadtake:"), 10, 64)
		if err != nil || number <= 0 {
			return b.showAdminLeads(ctx, chatID)
		}
		lead, _, err := b.leads.takeInWork(number, from.ID)
		if err != nil {
			return b.Send(ctx, chatID, "Не удалось изменить статус заявки: "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К заявкам", "adm:leads")}))
		}
		b.queueLeadDelivery(lead.Number)
		return b.showAdminLeads(ctx, chatID)
	case data == "adm:tools":
		return b.showAdminTools(ctx, chatID)
	case data == "adm:reload":
		if err := b.store.Reload(); err != nil {
			return b.Send(ctx, chatID, "Ошибка обновления каталога: "+html.EscapeString(err.Error()), nil)
		}
		return b.Send(ctx, chatID, "✅ Каталог обновлён.", markup([]Button{callbackButton("⬅️ Система", "adm:tools")}))
	case data == "adm:sync":
		return b.startManualSync(ctx, chatID)
	case data == "adm:status":
		items := b.store.List()
		geo, pics := 0, 0
		for _, l := range items {
			if l.Latitude != nil && l.Longitude != nil {
				geo++
			}
			if l.ImageURL != "" {
				pics++
			}
		}
		return b.Send(ctx, chatID, fmt.Sprintf("📍 <b>Состояние каталога</b>\nЛокаций: %d\nКоординаты: %d\nФото: %d", len(items), geo, pics), markup([]Button{callbackButton("⬅️ Система", "adm:tools")}))
	case strings.HasPrefix(data, "adm:stats:") && len(parts) == 4:
		period, err := strconv.Atoi(parts[3])
		if err != nil {
			period = 30
		}
		return b.showAdminStats(ctx, chatID, parts[2], period)
	case strings.HasPrefix(data, "adm:locations:"):
		session.Field = ""
		page, err := strconv.Atoi(strings.TrimPrefix(data, "adm:locations:"))
		if err != nil {
			page = 0
		}
		return b.showAdminLocations(ctx, chatID, page)
	case data == "adm:new":
		return b.startAdminNewLocation(ctx, chatID, from.ID)
	case strings.HasPrefix(data, "adm:edit:"):
		return b.startAdminEditor(ctx, chatID, from.ID, strings.TrimPrefix(data, "adm:edit:"))
	case data == "adm:locsearch":
		session.Field = "search_location"
		return b.Send(ctx, chatID, "🔎 <b>Поиск локации</b>\n\nВведите часть названия локации, которую хотите изменить.", markup([]Button{callbackButton("⬅️ К каталогу", "adm:locations:0")}))
	case data == "adm:editor":
		session.Field = ""
		return b.showAdminEditor(ctx, chatID, session)
	case data == "adm:sections":
		session.Field = ""
		return b.showAdminSections(ctx, chatID, session)
	case data == "adm:preview":
		session.Field = ""
		return b.renderAdminPreview(ctx, chatID, session)
	case data == "adm:publish":
		session.Field = ""
		return b.publishAdminDraft(ctx, chatID, from.ID, session)
	case strings.HasPrefix(data, "adm:field:"):
		field := strings.TrimPrefix(data, "adm:field:")
		switch field {
		case "title", "line_status", "landmarks", "map_url", "description", "category":
		default:
			return nil
		}
		session.Field = field
		return b.showAdminFieldPrompt(ctx, chatID, session, field)
	case strings.HasPrefix(data, "adm:section:"):
		n, err := strconv.Atoi(strings.TrimPrefix(data, "adm:section:"))
		if err != nil {
			return nil
		}
		return b.showAdminSection(ctx, chatID, session, n)
	case strings.HasPrefix(data, "adm:sectiontitle:") || strings.HasPrefix(data, "adm:sectionbody:"):
		n := 0
		field := ""
		if strings.HasPrefix(data, "adm:sectiontitle:") {
			n, _ = strconv.Atoi(strings.TrimPrefix(data, "adm:sectiontitle:"))
			field = "section_title"
		} else {
			n, _ = strconv.Atoi(strings.TrimPrefix(data, "adm:sectionbody:"))
			field = "section_body"
		}
		if n < 0 || n >= len(session.Draft.Sections) {
			return b.showAdminSections(ctx, chatID, session)
		}
		session.Field = field
		session.SectionIndex = n
		return b.showAdminFieldPrompt(ctx, chatID, session, field)
	case data == "adm:add":
		if len(session.Draft.Sections) >= 15 {
			return b.Send(ctx, chatID, "Достигнут предел 15 разделов.", nil)
		}
		index := len(session.Draft.Sections)
		session.Draft.Sections = append(session.Draft.Sections, catalog.CardSection{ID: fmt.Sprintf("extra-%d", time.Now().UTC().UnixNano()), Title: "Новый раздел", Body: "Добавьте описание", Enabled: true})
		return b.showAdminSection(ctx, chatID, session, index)
	case strings.HasPrefix(data, "adm:removeconfirm:"):
		n, err := strconv.Atoi(strings.TrimPrefix(data, "adm:removeconfirm:"))
		if err != nil || n < 0 || n >= len(session.Draft.Sections) {
			return b.showAdminSections(ctx, chatID, session)
		}
		session.Draft.Sections = append(session.Draft.Sections[:n], session.Draft.Sections[n+1:]...)
		return b.showAdminSections(ctx, chatID, session)
	case strings.HasPrefix(data, "adm:toggle:") || strings.HasPrefix(data, "adm:remove:") || strings.HasPrefix(data, "adm:up:") || strings.HasPrefix(data, "adm:down:"):
		op := parts[1]
		n, err := strconv.Atoi(parts[2])
		if err != nil || n < 0 || n >= len(session.Draft.Sections) {
			return nil
		}
		switch op {
		case "toggle":
			session.Draft.Sections[n].Enabled = !session.Draft.Sections[n].Enabled
		case "remove":
			return b.Send(ctx, chatID, "🗑 <b>Удалить раздел из черновика?</b>\n\n"+html.EscapeString(session.Draft.Sections[n].Title)+"\n\nРаздел будет удалён после публикации. До публикации можно отменить черновик, после публикации восстановить через историю.", markup(
				[]Button{callbackButton("🗑 Да, удалить", fmt.Sprintf("adm:removeconfirm:%d", n))},
				[]Button{callbackButton("❌ Отмена", fmt.Sprintf("adm:section:%d", n))},
			))
		case "up":
			if n > 0 {
				session.Draft.Sections[n], session.Draft.Sections[n-1] = session.Draft.Sections[n-1], session.Draft.Sections[n]
			}
			n--
		case "down":
			if n+1 < len(session.Draft.Sections) {
				session.Draft.Sections[n], session.Draft.Sections[n+1] = session.Draft.Sections[n+1], session.Draft.Sections[n]
			}
			n++
		}
		return b.showAdminSections(ctx, chatID, session)
	}
	return nil
}
func (b *Bot) handleAdminText(ctx context.Context, chatID int64, from user, input string) (bool, error) {
	if !b.isAdmin(from.ID) || chatID != from.ID {
		return false, nil
	}
	session := b.adminSession(from.ID)
	if session.Field == "search_location" {
		if strings.HasPrefix(input, "/") {
			return false, nil
		}
		matches := catalog.Search(b.store.List(), input)
		rows := [][]Button{}
		count := min(12, len(matches))
		for i := 0; i < count; i += 2 {
			row := []Button{callbackButton(buttonLabel(matches[i].Title), "adm:edit:"+matches[i].ID)}
			if i+1 < count {
				row = append(row, callbackButton(buttonLabel(matches[i+1].Title), "adm:edit:"+matches[i+1].ID))
			}
			rows = append(rows, row)
		}
		rows = append(rows, []Button{callbackButton("⬅️ К каталогу", "adm:locations:0")})
		if len(matches) == 0 {
			return true, b.Send(ctx, chatID, "Совпадений не найдено. Введите другой запрос.", markup(rows...))
		}
		session.Field = ""
		return true, b.Send(ctx, chatID, "🔎 <b>Найденные локации</b>\n\nВыберите карточку:", markup(rows...))
	}
	if session.Field == "new_title" && session.CreatingLocation {
		if strings.HasPrefix(input, "/") {
			return false, nil
		}
		input = strings.TrimSpace(input)
		draft := catalog.CardRevision{Title: input, Sections: []catalog.CardSection{}}
		if err := draft.Validate(); err != nil {
			return true, b.Send(ctx, chatID, "Некорректное название: "+html.EscapeString(err.Error())+"\nВведите название ещё раз.", markup([]Button{callbackButton("❌ Отмена", "adm:locations:0")}))
		}
		session.Draft = draft
		session.Field = ""
		return true, b.showAdminEditor(ctx, chatID, session)
	}
	if session.Field == "" || session.LocationID == "" {
		return false, nil
	}
	if strings.HasPrefix(input, "/") {
		return false, nil
	}
	input = strings.TrimSpace(input)
	oldDraft := session.Draft
	oldDraft.Sections = append([]catalog.CardSection(nil), session.Draft.Sections...)
	if len([]rune(input)) > 3500 {
		return true, b.Send(ctx, chatID, "Слишком длинный текст (максимум 3500 символов).", nil)
	}
	switch session.Field {
	case "title":
		session.Draft.Title = input
	case "line_status":
		session.Draft.LineStatus = input
	case "landmarks":
		session.Draft.Landmarks = input
	case "map_url":
		session.Draft.MapURL = input
	case "description":
		session.Draft.Description = input
	case "category":
		session.Draft.Category = input
	case "section_title":
		session.Draft.Sections[session.SectionIndex].Title = input
	case "section_body":
		session.Draft.Sections[session.SectionIndex].Body = input
	default:
		return false, nil
	}
	if err := session.Draft.Validate(); err != nil {
		session.Draft = oldDraft
		return true, b.Send(ctx, chatID, "Некорректное значение: "+html.EscapeString(err.Error()), nil)
	}
	session.Field = ""
	return true, b.showAdminEditor(ctx, chatID, session)
}
