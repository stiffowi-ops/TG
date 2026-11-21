package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"locationbot/internal/analytics"
	"locationbot/internal/catalog"
	"locationbot/internal/syncer"
)

type Bot struct {
	token, mapURL, offsetFile string
	store                     *catalog.Store
	favorites                 *favoriteStore
	syncer                    *syncer.Syncer
	adminIDs                  map[int64]bool
	leads                     *leadStore
	http                      *http.Client
	compareMu                 sync.Mutex
	compareSelections         map[int64][]string
	screenMu                  sync.Mutex
	lastScreens               map[int64]int
	searchPrompts             map[int64]int
	botID                     int64
	botIdentityMu sync.RWMutex
	botUsername string
	referralWake chan struct{}
	metrics                   *analytics.Store
	adminMu                   sync.Mutex
	backupMu                  sync.Mutex
	adminStates               map[int64]*adminSession
	visitors                  *visitorStore
	leadNotifyMu              sync.Mutex
	deliveryMu                sync.Mutex
	refreshCases              map[int64]bool
	deliveryWake              chan struct{}
	keyboardCleanup           chan int64
	manualSync                sync.Mutex
}
type user struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}
type chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
type geoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type message struct {
	MessageID int               `json:"message_id"`
	Text      string            `json:"text"`
	Chat      chat              `json:"chat"`
	From      user              `json:"from"`
	ViaBot    *user             `json:"via_bot"`
	Location  *geoPoint         `json:"location"`
	Document  *telegramDocument `json:"document"`
}
type callbackQuery struct {
	ID              string   `json:"id"`
	Data            string   `json:"data"`
	From            user     `json:"from"`
	Message         *message `json:"message"`
	InlineMessageID string   `json:"inline_message_id"`
}
type update struct {
	UpdateID    int64               `json:"update_id"`
	Message     *message            `json:"message"`
	Callback    *callbackQuery      `json:"callback_query"`
	InlineQuery *inlineQuery        `json:"inline_query"`
	Chosen      *chosenInlineResult `json:"chosen_inline_result"`
}
type telegramDocument struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}
type chosenInlineResult struct {
	ResultID string `json:"result_id"`
	From     user   `json:"from"`
}
type apiResult struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}
type Button map[string]any

func urlButton(title, dest string) Button      { return Button{"text": title, "url": dest} }
func callbackButton(title, data string) Button { return Button{"text": title, "callback_data": data} }
func markup(rows ...[]Button) map[string]any   { return map[string]any{"inline_keyboard": rows} }
func menuButton() Button                       { return callbackButton("🏠 Главное меню", "menu") }
func homeMarkup() any                          { return markup([]Button{menuButton()}) }

// Keep every navigation action inline. Requesting a location from an inline
// button is not supported by Telegram: users send their location via Attach.
func startMenuMarkup(mapURL string, action Button, admin bool) map[string]any {
	rows := [][]Button{
		{callbackButton("📋 Список локаций", "list"), urlButton("🗺 Карта локаций", mapURL)},
		{callbackButton("🔎 Поиск локации", "search:menu"), callbackButton("📍 Рядом со мной", "near")},
		{callbackButton("⭐ Избранное", "favorites"), cabinetButton()},
		{action},
	}
	if appURL:=strings.TrimSpace(os.Getenv("MINIAPP_URL")); strings.HasPrefix(appURL,"https://") {
		rows=append([][]Button{{Button{"text":"Открыть приложение","web_app":map[string]string{"url":appURL}}}},rows...)
	}
	if admin {
		rows = append(rows, []Button{callbackButton("⚙️ Админ-панель", "adm:menu")})
	}
	return markup(rows...)
}

const defaultProjectMapURL = "https://yandex.ru/maps/?um=constructor%3A4e091ad95afb487039d07dc0667d6d596da173fe30cff48c7f2e234b7150d7f0&source=constructorLink"

func New(token, constructorID, dataDir string, store *catalog.Store, sy *syncer.Syncer, admins string) (*Bot, error) {
	ids := map[int64]bool{}
	for _, value := range strings.Split(admins, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && id > 0 {
			ids[id] = true
		}
	}
	mapURL := defaultProjectMapURL
	if constructorID = strings.TrimSpace(constructorID); constructorID != "" {
		mapURL = "https://yandex.ru/maps/?um=" + url.QueryEscape("constructor:"+constructorID) + "&source=constructorLink"
	}
	favorites, err := openFavoriteStore(filepath.Join(dataDir, "favorites.json"))
	if err != nil {
		return nil, err
	}
	leads, err := openLeadStore(filepath.Join(dataDir, "leads.json"))
	if err != nil {
		return nil, err
	}
	metrics, err := analytics.Open(filepath.Join(dataDir, "analytics.jsonl"))
	if err != nil {
		return nil, err
	}
	visitors, err := openVisitorStore(filepath.Join(dataDir, "visitors.json"))
	if err != nil {
		return nil, err
	}
	return &Bot{
		token: token, mapURL: mapURL, store: store, favorites: favorites, syncer: sy,
		adminIDs: ids, leads: leads, metrics: metrics, visitors: visitors, http: &http.Client{Timeout: 50 * time.Second},
		offsetFile:        filepath.Join(dataDir, "telegram-offset.txt"),
		compareSelections: make(map[int64][]string),
		deliveryWake:      make(chan struct{}, 1),
		keyboardCleanup:   make(chan int64, 64),
		referralWake: make(chan struct{},1),
	}, nil
}
func (b *Bot) call(ctx context.Context, method string, payload any) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+b.token+"/"+method, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var result apiResult
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("Telegram %s: %s", method, result.Description)
	}
	return result.Result, nil
}
func (b *Bot) Send(ctx context.Context, chatID int64, text string, keyboard any) error {
	if keyboard == nil {
		keyboard = homeMarkup()
	}
	if inlineID, ok := inlineNavigationFromContext(ctx); ok {
		if err := b.editInlineNavigationMessage(ctx, inlineID, text, keyboard); err == nil {
			return nil
		} else {
			log.Printf("Edit inline screen %s: %v; sending private fallback", inlineID, err)
			return b.sendNewText(ctx, chatID, text, keyboard)
		}
	}
	if messageID, ok := navigationFromContext(ctx, chatID); ok {
		return b.editTextOrSend(ctx, chatID, messageID, text, keyboard)
	}
	return b.sendNewText(ctx, chatID, text, keyboard)
}
func (b *Bot) answerCallback(ctx context.Context, id string) {
	_, _ = b.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id})
}
func welcomeText(sender user) string {
	name := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(sender.FirstName), strings.TrimSpace(sender.LastName)}, " "))
	greeting := "Привет! 👋"
	if name != "" {
		greeting = "Привет, " + html.EscapeString(name) + "! 👋"
	}
	return greeting + "\n\n🚀 <b>Не просто точки на карте. Точки роста твоего бизнеса!</b>\n\nМы собрали список локаций с интересными возможностями для открытия и развития бизнеса.\n\nИзучай расположение, потенциальную аудиторию и преимущества каждой точки. Сравнивай варианты и сохраняй лучшие в избранное ⭐\n\n<b>Найди свою следующую точку роста 👇</b>"
}

// Deliver the requested screen first. Legacy keyboard migration is best effort
// and must never make a user wait for two extra Telegram calls.
func (b *Bot) sendInlineAndRemoveReplyKeyboard(ctx context.Context, chatID int64, text string, inline any) error {
	if err := b.Send(ctx, chatID, text, inline); err != nil {
		return err
	}
	if _, ok := navigationFromContext(ctx, chatID); ok {
		return nil
	}
	if _, ok := inlineNavigationFromContext(ctx); ok {
		return nil
	}
	select {
	case b.keyboardCleanup <- chatID:
	default: // Retry on the next visit if the migration worker is busy.
	}
	return nil
}

func (b *Bot) welcome(ctx context.Context, chatID int64, sender user) error {
	if _, shared := inlineNavigationFromContext(ctx); shared {
		return b.Send(ctx, chatID, welcomeText(sender), startMenuMarkup(b.mapURL, leadButton("menu", ""), false))
	}
	return b.sendInlineAndRemoveReplyKeyboard(ctx, chatID, welcomeText(sender), b.adminHomeMarkup(b.mapURL, sender.ID))
}

// Two columns on every page. Sort by the real station name, not "Метро" or image ID.
const stationPageSize = 10

func stationLabel(title string) string {
	title = publicText(title)
	if strings.HasPrefix(strings.ToLower(title), "метро ") {
		return strings.TrimSpace(string([]rune(title)[len([]rune("Метро ")):]))
	}
	return title
}

func stationSortKey(title string) string {
	return strings.ToLower(strings.ReplaceAll(stationLabel(title), "ё", "е"))
}

func sortedStations(items []catalog.Location) []catalog.Location {
	out := append([]catalog.Location(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := stationSortKey(out[i].Title), stationSortKey(out[j].Title)
		if a == b {
			return out[i].ID < out[j].ID
		}
		return a < b
	})
	return out
}

func limitRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if max <= 0 {
		return ""
	}
	if len(r) <= max {
		return string(r)
	}
	if max == 1 {
		return "…"
	}
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

func buttonLabel(title string) string {
	// Keep labels compact on mobile; full name remains in the station card.
	return limitRunes(stationLabel(title), 22)
}

func stationPageRows(items []catalog.Location, page int) ([][]Button, int) {
	stations := sortedStations(items)
	lastPage := 0
	if len(stations) > 0 {
		lastPage = (len(stations) - 1) / stationPageSize
	}
	if page < 0 {
		page = 0
	}
	if page > lastPage {
		page = lastPage
	}
	start := page * stationPageSize
	end := min(start+stationPageSize, len(stations))
	rows := make([][]Button, 0, 8)
	for i := start; i < end; i += 2 {
		row := []Button{callbackButton(buttonLabel(stations[i].Title), fmt.Sprintf("loc:%s:%d", stations[i].ID, page))}
		if i+1 < end {
			row = append(row, callbackButton(buttonLabel(stations[i+1].Title), fmt.Sprintf("loc:%s:%d", stations[i+1].ID, page)))
		}
		rows = append(rows, row)
	}
	nav := []Button{}
	if page > 0 {
		nav = append(nav, callbackButton("◀️ Назад", fmt.Sprintf("page:%d", page-1)))
	}
	if page < lastPage {
		nav = append(nav, callbackButton("Далее ▶️", fmt.Sprintf("page:%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{callbackButton("🔎 Найти локацию", "search:catalog"), leadButton("menu", "")}, []Button{menuButton()})
	return rows, lastPage
}

func (b *Bot) showPage(ctx context.Context, chatID int64, page int) error {
	items := b.store.List()
	rows, maxPage := stationPageRows(items, page)
	if page < 0 {
		page = 0
	}
	if page > maxPage {
		page = maxPage
	}
	return b.Send(ctx, chatID, fmt.Sprintf("<b>📍 Каталог локаций</b>\n\nИзучите доступные локации и выберите подходящую для Вашего бизнеса. Откройте интересующую точку, а понравившиеся варианты сохраните в избранное.\n\n📖 Страница %d из %d", page+1, maxPage+1), markup(rows...))
}

// Telegram text messages are limited to 4096 characters; keep the complete
// station template together with one image album wherever possible.
const cardTextBudget = 3700

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }

func limitCaptionUnits(s string, maxUnits int) string {
	s = strings.TrimSpace(s)
	if maxUnits <= 0 {
		return ""
	}
	if utf16Len(s) <= maxUnits {
		return s
	}
	if maxUnits == 1 {
		return "…"
	}
	units := 0
	r := []rune{}
	for _, ch := range s {
		step := 1
		if ch > 0xFFFF {
			step = 2
		}
		if units+step > maxUnits-1 {
			break
		}
		r = append(r, ch)
		units += step
	}
	return strings.TrimSpace(string(r)) + "…"
}

// stationMapLink never silently redirects a station to the general map.
// An explicit Yandex link copied from the source project takes priority.
// Otherwise, verified coordinates center the original Constructor layer and
// place a pin at the exact coordinate. Without coordinates, use the project map.
func validatedYandexMapURL(raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) >= 1500 {
		return nil, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return nil, false
	}
	host := strings.ToLower(u.Hostname())
	if host != "yandex.ru" && !strings.HasSuffix(host, ".yandex.ru") && host != "yandex.com" && !strings.HasSuffix(host, ".yandex.com") {
		return nil, false
	}
	return u, true
}

func stationMapLink(l catalog.Location, originalLayerURL string) string {
	if direct := strings.TrimSpace(l.MapURL); direct != "" {
		if _, ok := validatedYandexMapURL(direct); ok {
			return direct
		}
	}
	layer, ok := validatedYandexMapURL(originalLayerURL)
	if !ok {
		return ""
	}
	if l.Latitude == nil || l.Longitude == nil {
		return layer.String()
	}
	lat, lon := *l.Latitude, *l.Longitude
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return layer.String()
	}
	coordinates := fmt.Sprintf("%.7f,%.7f", lon, lat)
	q := layer.Query()
	q.Set("ll", coordinates)
	q.Set("pt", coordinates)
	q.Set("z", "16")
	q.Set("l", "map")
	layer.RawQuery = q.Encode()
	return layer.String()
}

// cardField renders a safe, consistent placeholder for the 39 not-yet-filled
// partner cards. It never asserts facts about an unverified station.
func cardField(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Информация пока не добавлена."
	}
	return limitCaptionUnits(strings.TrimSpace(s), 620)
}

// Present the location as a compact Telegram Rich Text card.
// The reference work bot uses Telegram HTML with clear headings, whitespace
// and a lightweight divider; MarkdownV2 is not required to achieve the style.
// No photo is sent. This formatter is shared by catalog and inline search.
func cardForLocation(l catalog.Location, layerURL string) string {
	l = publicLocation(l)
	title := html.EscapeString(limitCaptionUnits(l.Title, 150))
	isMetro := strings.HasPrefix(strings.ToLower(strings.TrimSpace(l.Title)), "метро ")
	icon := "📍"
	statusIcon := "📌"
	if isMetro {
		icon = "🚇"
		statusIcon = "🚇"
	}

	status := strings.TrimSpace(l.LineStatus)
	if status == "" {
		status = "Статус пока не указан."
	}
	landmark := strings.TrimSpace(l.Landmarks)
	if landmark == "" {
		landmark = "Пока не указан."
	}
	traffic := l.Traffic
	if strings.TrimSpace(traffic) == "" && strings.TrimSpace(l.Description) != "" {
		traffic = l.Description
	}
	mapLine := "Ссылка на карту пока не настроена."
	if link := stationMapLink(l, layerURL); link != "" {
		mapLine = `<a href="` + html.EscapeString(link) + `">Открыть карту</a>`
	}

	lines := []string{
		icon + " <b>" + title + "</b>",
		statusIcon + " <i>" + html.EscapeString(limitCaptionUnits(status, 200)) + "</i>",
		"📌 <b>Ориентир:</b> " + html.EscapeString(limitCaptionUnits(landmark, 310)),
		"",
		"━━━━━━━━━━━━━━",
		"",
		"🚶 <b>Трафик и локация</b>",
		html.EscapeString(cardField(traffic)),
		"",
		"👥 <b>Кто платит</b>",
		html.EscapeString(cardField(l.Payers)),
		"",
		"📈 <b>Возможности для бизнеса</b>",
		html.EscapeString(cardField(l.Opportunity)),
		"",
		"🎯 <b>Почему именно здесь</b>",
		html.EscapeString(cardField(l.Conclusion)),
	}
	if rating := strings.TrimSpace(l.Rating); rating != "" {
		lines = append(lines, "", "⭐ <b>Оценка:</b> "+html.EscapeString(limitCaptionUnits(rating, 100)))
	}
	lines = append(lines, "", "🗺 <b>Локация на карте:</b> "+mapLine)
	return strings.Join(lines, "\n")
}

func locationBackRows(backPage int) [][]Button {
	if backPage < 0 {
		backPage = 0
	}
	return [][]Button{
		{callbackButton("⬅️ К списку", fmt.Sprintf("page:%d", backPage)), callbackButton("🔎 Поиск локации", "search:catalog")},
		{menuButton()},
	}
}

// Return navigation matches the section where the search was opened.
// "Начать поиск" invokes Telegram's live inline suggestions; the surrounding
// navigation remains a normal callback keyboard that can be edited in place.
func (b *Bot) requestStationSearch(ctx context.Context, chatID int64, origin ...string) error {
	source := "menu"
	if len(origin) > 0 {
		source = origin[0]
	}
	back := callbackButton("📋 Список локаций", "list")
	leadSource := "menu"
	switch source {
	case "favorites":
		back = callbackButton("⭐ К избранному", "favorites")
		leadSource = "favorites"
	case "catalog":
		back = callbackButton("⬅️ К списку", "page:0")
	}
	search := inlineSearchButton()
	search["text"] = "🔎 Начать поиск"
	err := b.Send(ctx, chatID, "🔎 <b>Поиск локации</b>\n\nНажмите «Начать поиск» и введите название. Подходящие варианты появятся по мере ввода. Точное название необязательно: достаточно его части.\n\nВы также можете отправить название обычным сообщением.", markup(
		[]Button{search, back},
		[]Button{leadButton(leadSource, ""), menuButton()},
	))
	if err == nil {
		if _, inline := inlineNavigationFromContext(ctx); !inline {
			b.markSearchPrompt(chatID)
		}
	}
	return err
}
func (b *Bot) showLocation(ctx context.Context, chatID, userID int64, id string, backPage int) error {
	l, found := b.store.Find(id)
	if !found {
		return b.Send(ctx, chatID, "Локация не найдена. Откройте список локаций.", markup(locationBackRows(backPage)...))
	}
	saved := b.favorites != nil && b.favorites.Contains(userID, id)
	return b.sendRichLocation(ctx, chatID, l, markup(regularCardRows(id, backPage, saved)...))
}

func (b *Bot) favoriteLocations(userID int64) []catalog.Location {
	items := make([]catalog.Location, 0)
	for _, id := range b.favorites.IDs(userID) {
		if item, ok := b.store.Find(id); ok {
			items = append(items, item)
		}
	}
	return sortedStations(items)
}

func (b *Bot) showFavorites(ctx context.Context, chatID, userID int64) error {
	items := b.favoriteLocations(userID)
	if len(items) == 0 {
		text := "⭐ <b>Избранное</b>\n\nПока здесь пусто — сохраним первую локацию? 🙂\n\nОткройте список, выберите интересное место и нажмите «⭐ Добавить в избранное» в карточке локации. Сохранённые локации появятся здесь — к ним можно будет вернуться или сравнить варианты."
		return b.Send(ctx, chatID, text, markup(
			[]Button{callbackButton("📋 Выбрать локацию", "list"), callbackButton("🔎 Найти локацию", "search:favorites")},
			[]Button{leadButton("favorites", "")},
			[]Button{menuButton()},
		))
	}
	rows := make([][]Button, 0, len(items)+3)
	for _, item := range items {
		rows = append(rows, []Button{
			callbackButton("🚇 "+buttonLabel(item.Title), "loc:"+item.ID),
			callbackButton("✖️ Убрать", "fav:remove:"+item.ID),
		})
	}
	rows = append(rows,
		[]Button{callbackButton("⚖️ Сравнить локации", "compare:start")},
		[]Button{callbackButton("📋 Добавить локацию", "list"), callbackButton("🔎 Найти локацию", "search:favorites")},
		[]Button{leadButton("favorites", "")},
		[]Button{menuButton()},
	)
	text := fmt.Sprintf("⭐ <b>Избранное</b> · %d\n\nОткройте локацию, уберите её из списка или выберите несколько для сравнения.", len(items))
	return b.Send(ctx, chatID, text, markup(rows...))
}

func (b *Bot) comparisonSelection(userID int64) []string {
	b.compareMu.Lock()
	defer b.compareMu.Unlock()
	return append([]string(nil), b.compareSelections[userID]...)
}

func (b *Bot) clearComparisonSelection(userID int64) {
	b.compareMu.Lock()
	defer b.compareMu.Unlock()
	delete(b.compareSelections, userID)
}

func (b *Bot) removeComparisonSelection(userID int64, stationID string) {
	b.compareMu.Lock()
	defer b.compareMu.Unlock()
	selected := b.compareSelections[userID]
	next := make([]string, 0, len(selected))
	for _, id := range selected {
		if id != stationID {
			next = append(next, id)
		}
	}
	if len(next) == 0 {
		delete(b.compareSelections, userID)
	} else {
		b.compareSelections[userID] = next
	}
}

// toggleComparisonSelection returns the current count and whether the new ID
// was rejected because the three-station limit had already been reached.
func (b *Bot) toggleComparisonSelection(userID int64, stationID string) (count int, limitReached bool) {
	b.compareMu.Lock()
	defer b.compareMu.Unlock()
	if b.compareSelections == nil {
		b.compareSelections = make(map[int64][]string)
	}
	selected := b.compareSelections[userID]
	for i, id := range selected {
		if id == stationID {
			selected = append(selected[:i], selected[i+1:]...)
			if len(selected) == 0 {
				delete(b.compareSelections, userID)
			} else {
				b.compareSelections[userID] = selected
			}
			return len(selected), false
		}
	}
	if len(selected) >= 3 {
		return len(selected), true
	}
	b.compareSelections[userID] = append(selected, stationID)
	return len(selected) + 1, false
}

func (b *Bot) editTextOrSend(ctx context.Context, chatID int64, messageID int, text string, keyboard any) error {
	if messageID != 0 {
		_, err := b.call(ctx, "editMessageText", map[string]any{
			"chat_id": chatID, "message_id": messageID, "text": text,
			"parse_mode": "HTML", "link_preview_options": map[string]any{"is_disabled": true},
			"reply_markup": keyboard,
		})
		if err == nil || strings.Contains(strings.ToLower(err.Error()), "message is not modified") {
			b.rememberBotScreenID(chatID, messageID)
			return nil
		}
		log.Printf("Refresh navigation message %d: %v; sending a new message", messageID, err)
	}
	// Do not call Send with the same navigation context here. That would
	// retry the failing edit recursively instead of creating a fallback.
	return b.sendNewText(ctx, chatID, text, keyboard)
}

func (b *Bot) showComparePicker(ctx context.Context, chatID, userID int64, messageID int, notice string) error {
	items := b.favoriteLocations(userID)
	if len(items) < 2 {
		return b.Send(ctx, chatID, "Чтобы сравнить локации, сначала добавьте в избранное хотя бы две.", markup(
			[]Button{callbackButton("📋 Выбрать локацию", "list"), callbackButton("🔎 Найти локацию", "search:favorites")},
			[]Button{leadButton("favorites", "")},
			[]Button{menuButton()},
		))
	}
	selected := b.comparisonSelection(userID)
	selectedSet := make(map[string]bool, len(selected))
	for _, id := range selected {
		selectedSet[id] = true
	}
	rows := make([][]Button, 0, len(items)+2)
	for _, item := range items {
		mark := "▫️ "
		if selectedSet[item.ID] {
			mark = "✅ "
		}
		rows = append(rows, []Button{callbackButton(mark+buttonLabel(item.Title), "compare:toggle:"+item.ID)})
	}
	if len(selected) >= 2 {
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("⚖️ Сравнить выбранные · %d", len(selected)), "compare:done")})
	}
	rows = append(rows, []Button{callbackButton("⬅️ В избранное", "favorites"), menuButton()})
	text := fmt.Sprintf("⚖️ <b>Сравнение локаций</b>\n\nВыберите 2–3 локации из избранного. Отмечено: %d/3.", len(selected))
	if notice != "" {
		text += "\n\n" + html.EscapeString(notice)
	}
	return b.editTextOrSend(ctx, chatID, messageID, text, markup(rows...))
}

func compareField(label, value string) string {
	if strings.TrimSpace(value) == "" {
		value = "Информация пока не добавлена."
	}
	return label + html.EscapeString(limitCaptionUnits(publicText(value), 120))
}

func comparisonText(items []catalog.Location) string {
	for i := range items {
		items[i] = publicLocation(items[i])
	}
	var lines []string
	lines = append(lines, "⚖️ <b>Сравнение локаций</b>")
	for i, item := range items {
		traffic := item.Traffic
		if strings.TrimSpace(traffic) == "" {
			traffic = item.Description
		}
		status := strings.TrimSpace(item.LineStatus)
		if status == "" {
			status = "Пока не указан."
		}
		landmark := strings.TrimSpace(item.Landmarks)
		if landmark == "" {
			landmark = "Пока не указан."
		}
		lines = append(lines,
			"",
			fmt.Sprintf("<b>%d. 🚇 %s</b>", i+1, html.EscapeString(limitCaptionUnits(item.Title, 100))),
			compareField("🚇 Линия и статус: ", status),
			compareField("📍 Ориентир: ", landmark),
			compareField("🚶 Трафик: ", traffic),
			compareField("👥 Кто платит: ", item.Payers),
			compareField("📈 Возможность: ", item.Opportunity),
			compareField("🎯 Почему здесь: ", item.Conclusion),
		)
	}
	lines = append(lines, "", "Нажмите на локацию ниже, чтобы открыть её карточку и карту.")
	return strings.Join(lines, "\n")
}

func (b *Bot) finishComparison(ctx context.Context, chatID, userID int64) error {
	selected := b.comparisonSelection(userID)
	if len(selected) < 2 || len(selected) > 3 {
		return b.showComparePicker(ctx, chatID, userID, 0, "Выберите минимум две локации, чтобы сравнить варианты.")
	}
	items := make([]catalog.Location, 0, len(selected))
	for _, id := range selected {
		if !b.favorites.Contains(userID, id) {
			continue
		}
		if item, ok := b.store.Find(id); ok {
			items = append(items, item)
		}
	}
	b.clearComparisonSelection(userID)
	if len(items) < 2 {
		return b.showFavorites(ctx, chatID, userID)
	}
	rows := make([][]Button, 0, len(items)+1)
	for _, item := range items {
		rows = append(rows, []Button{callbackButton("🚇 "+buttonLabel(item.Title), "loc:"+item.ID)})
	}
	rows = append(rows, []Button{callbackButton("⭐ Избранное", "favorites"), menuButton()})
	return b.Send(ctx, chatID, comparisonText(items), markup(rows...))
}

func (b *Bot) startComparison(ctx context.Context, chatID, userID int64, messageID int) error {
	if len(b.favoriteLocations(userID)) < 2 {
		return b.showComparePicker(ctx, chatID, userID, 0, "")
	}
	b.clearComparisonSelection(userID)
	return b.showComparePicker(ctx, chatID, userID, messageID, "")
}

type Nearby struct {
	Location   catalog.Location
	DistanceKM float64
}

func Nearest(loc []catalog.Location, lat, lon float64, limit int) []Nearby {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || math.IsNaN(lat) || math.IsNaN(lon) || limit <= 0 {
		return nil
	}
	res := []Nearby{}
	for _, l := range loc {
		if l.Latitude == nil || l.Longitude == nil {
			continue
		}
		a, b := *l.Latitude, *l.Longitude
		if a < -90 || a > 90 || b < -180 || b > 180 || math.IsNaN(a) || math.IsNaN(b) {
			continue
		}
		res = append(res, Nearby{Location: l, DistanceKM: distanceKM(lat, lon, a, b)})
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].DistanceKM == res[j].DistanceKM {
			return res[i].Location.ID < res[j].Location.ID
		}
		return res[i].DistanceKM < res[j].DistanceKM
	})
	if len(res) > limit {
		res = res[:limit]
	}
	return res
}
func distanceKM(aLat, aLon, bLat, bLon float64) float64 {
	const radius = 6371.0088
	rad := math.Pi / 180
	latDelta := (bLat - aLat) * rad
	lonDelta := (bLon - aLon) * rad
	h := math.Pow(math.Sin(latDelta/2), 2) + math.Cos(aLat*rad)*math.Cos(bLat*rad)*math.Pow(math.Sin(lonDelta/2), 2)
	return 2 * radius * math.Asin(math.Sqrt(math.Min(1, h)))
}
func (b *Bot) showNear(ctx context.Context, chatID int64, p *geoPoint) error {
	if p == nil {
		return nil
	}
	near := Nearest(b.store.List(), p.Latitude, p.Longitude, 5)
	if len(near) == 0 {
		return b.Send(ctx, chatID, "Пока не могу определить ближайшую локацию: в каталоге ещё нет проверенных координат. Для поиска по геопозиции необходим экспорт точек из исходной карты. Поиск по названию уже работает.", markup([]Button{callbackButton("📋 Список локаций", "page:0"), menuButton()}))
	}
	rows := [][]Button{}
	for i, n := range near {
		rows = append(rows, []Button{callbackButton(fmt.Sprintf("%d. %s · %.1f км", i+1, stationLabel(n.Location.Title), n.DistanceKM), "loc:"+n.Location.ID)})
	}
	rows = append(rows, []Button{callbackButton("⬅️ К списку", "page:0"), menuButton()})
	return b.Send(ctx, chatID, "<b>Ближайшие локации</b> 📍\nРасстояния рассчитаны <b>по прямой</b>, а не по маршруту. Нажмите на локацию, чтобы посмотреть описание и оценку.\n\nГеопозицию бот не сохраняет.", markup(rows...))
}
func (b *Bot) handle(ctx context.Context, u update) error {
	if u.Chosen != nil {
		if id := strings.TrimPrefix(u.Chosen.ResultID, "karty:"); id != u.Chosen.ResultID {
			b.recordLocationAnalytics(u.Chosen.From.ID, id, analytics.View)
		}
		return nil
	}
	if u.InlineQuery != nil {
		return b.answerLocationInlineQuery(ctx, u.InlineQuery)
	}
	if u.Callback != nil {
		if u.Callback.Message != nil && u.Callback.Message.Chat.Type == "private" && u.Callback.Message.Chat.ID == u.Callback.From.ID && !strings.HasPrefix(u.Callback.Data, "adm:") {
			b.observeVisitor(u.Callback.From, b.visitorCallbackAction(u.Callback.Data), false)
		}
		if strings.HasPrefix(u.Callback.Data, "discount:") {
 return b.handleDiscountCallback(ctx,u.Callback)
 }
		if strings.HasPrefix(u.Callback.Data, "ref:") {
			b.answerCallback(ctx,u.Callback.ID)
			return b.handleReferralAdminCallback(ctx,u.Callback)
		}
		if strings.HasPrefix(u.Callback.Data, "lead:take:") || strings.HasPrefix(u.Callback.Data, "lead:done:") {
			b.answerCallback(ctx, u.Callback.ID)
			return b.handleLeadAdminCallback(ctx, u.Callback)
		}
		if strings.HasPrefix(u.Callback.Data, "adm:") {
			b.answerCallback(ctx, u.Callback.ID)
			if u.Callback.Message == nil || u.Callback.Message.Chat.Type != "private" || !b.isAdmin(u.Callback.From.ID) || u.Callback.Message.Chat.ID != u.Callback.From.ID {
				return nil
			}
			adminCtx := withNavigation(ctx, u.Callback.Message.Chat.ID, u.Callback.Message.MessageID)
			if strings.HasPrefix(u.Callback.Data, "adm:case:") {
				return b.handleCaseAdminCallback(adminCtx, u.Callback.From, u.Callback.Data)
			}
			return b.handleAdminCallback(adminCtx, u.Callback.From.ID, u.Callback.From, u.Callback.Data)
		}
		if strings.HasPrefix(u.Callback.Data, "fav:inline:") {
			return b.toggleInlineFavorite(ctx, u.Callback)
		}
		if strings.HasPrefix(u.Callback.Data, "inline:nav:") {
			return b.inlineNavigate(ctx, u.Callback)
		}
		b.answerCallback(ctx, u.Callback.ID)
		chatID := u.Callback.From.ID
		if u.Callback.Message != nil {
			chatID = u.Callback.Message.Chat.ID
			if u.Callback.Message.Chat.Type != "private" {
				return nil
			}
			ctx = withNavigation(ctx, chatID, u.Callback.Message.MessageID)
		} else {
			// Telegram sends inline_message_id (without Message) for results
			// chosen via inline search. Public navigation may edit that card.
			if u.Callback.InlineMessageID == "" || chatID <= 0 {
				return nil
			}
			if strings.HasPrefix(u.Callback.Data, "cab:") {
				return b.handleCabinetCallback(b.privateScreenContext(ctx, chatID), chatID, u.Callback.From, u.Callback.Data)
			}
			if u.Callback.Data == "favorites" {
				return b.showFavorites(b.privateScreenContext(ctx, chatID), chatID, u.Callback.From.ID)
			}
			if strings.HasPrefix(u.Callback.Data, "lead:") {
				return b.handleLeadCallback(b.privateScreenContext(ctx, chatID), chatID, u.Callback.From, u.Callback.Data)
			}
			if !supportsPublicInlineNavigation(u.Callback.Data) {
				return nil
			}
			ctx = withInlineNavigation(ctx, u.Callback.InlineMessageID)
		}
		if strings.HasPrefix(u.Callback.Data, "cab:") {
			return b.handleCabinetCallback(ctx, chatID, u.Callback.From, u.Callback.Data)
		}
		if strings.HasPrefix(u.Callback.Data, "lead:") {
			return b.handleLeadCallback(ctx, chatID, u.Callback.From, u.Callback.Data)
		}
		if err := b.pauseClientForms(u.Callback.From.ID); err != nil {
			return err
		}
		switch {
		case u.Callback.Data == "menu":
			return b.welcome(ctx, chatID, u.Callback.From)
		case u.Callback.Data == "near":
			return b.requestLocation(ctx, chatID)
		case u.Callback.Data == "list":
			return b.showPage(ctx, chatID, 0)
		case u.Callback.Data == "search" || strings.HasPrefix(u.Callback.Data, "search:"):
			origin := "menu"
			if strings.HasPrefix(u.Callback.Data, "search:") {
				origin = strings.TrimPrefix(u.Callback.Data, "search:")
			}
			return b.requestStationSearch(ctx, chatID, origin)
		case strings.HasPrefix(u.Callback.Data, "loc:"):
			parts := strings.Split(strings.TrimPrefix(u.Callback.Data, "loc:"), ":")
			if len(parts) == 0 {
				return nil
			}
			page := 0
			if len(parts) > 1 {
				page, _ = strconv.Atoi(parts[1])
			}
			err := b.showLocation(ctx, chatID, u.Callback.From.ID, parts[0], page)
			if err == nil {
				b.recordLocationAnalytics(u.Callback.From.ID, parts[0], analytics.View)
			}
			return err
		case strings.HasPrefix(u.Callback.Data, "page:"):
			n, _ := strconv.Atoi(strings.TrimPrefix(u.Callback.Data, "page:"))
			return b.showPage(ctx, chatID, n)
		case u.Callback.Data == "favorites":
			return b.showFavorites(ctx, chatID, u.Callback.From.ID)
		case u.Callback.Data == "compare:start":
			return b.startComparison(ctx, chatID, u.Callback.From.ID, u.Callback.Message.MessageID)
		case u.Callback.Data == "compare:done":
			return b.finishComparison(ctx, chatID, u.Callback.From.ID)
		case strings.HasPrefix(u.Callback.Data, "compare:toggle:"):
			stationID := strings.TrimPrefix(u.Callback.Data, "compare:toggle:")
			if !b.favorites.Contains(u.Callback.From.ID, stationID) {
				return b.showFavorites(ctx, chatID, u.Callback.From.ID)
			}
			_, full := b.toggleComparisonSelection(u.Callback.From.ID, stationID)
			notice := ""
			if full {
				notice = "Можно выбрать не больше трёх локаций."
			}
			return b.showComparePicker(ctx, chatID, u.Callback.From.ID, u.Callback.Message.MessageID, notice)
		case strings.HasPrefix(u.Callback.Data, "fav:toggle:"):
			parts := strings.SplitN(strings.TrimPrefix(u.Callback.Data, "fav:toggle:"), ":", 2)
			if len(parts) == 0 || parts[0] == "" {
				return nil
			}
			page := 0
			if len(parts) == 2 {
				page, _ = strconv.Atoi(parts[1])
			}
			added, err := b.favorites.Toggle(u.Callback.From.ID, parts[0])
			if err != nil {
				return err
			}
			if !added {
				b.removeComparisonSelection(u.Callback.From.ID, parts[0])
			}
			kind := analytics.FavoriteRemoved
			if added {
				kind = analytics.FavoriteAdded
			}
			b.recordLocationAnalytics(u.Callback.From.ID, parts[0], kind)
			return b.showLocation(ctx, chatID, u.Callback.From.ID, parts[0], page)
		case strings.HasPrefix(u.Callback.Data, "fav:remove:"):
			stationID := strings.TrimPrefix(u.Callback.Data, "fav:remove:")
			removed, err := b.favorites.Remove(u.Callback.From.ID, stationID)
			if err != nil {
				return err
			}
			b.removeComparisonSelection(u.Callback.From.ID, stationID)
			if removed {
				b.recordLocationAnalytics(u.Callback.From.ID, stationID, analytics.FavoriteRemoved)
			}
			return b.showFavorites(ctx, chatID, u.Callback.From.ID)
		}
		return nil
	}
	m := u.Message
	if m == nil || m.Chat.Type != "private" {
		return nil
	}
	if m.Document != nil && b.isAdmin(m.From.ID) && b.adminSession(m.From.ID).AwaitingBackup {
		if err := b.receiveAdminBackup(ctx, m.Chat.ID, m.From.ID, m.Document.FileID, m.Document.FileSize); err != nil {
			return b.Send(ctx, m.Chat.ID, "Не удалось загрузить архив. Проверьте формат и повторите попытку.", nil)
		}
		return nil
	}
	// An inline result is sent by the user via our bot. Its text already IS
	// the location card; do not treat it as a new name-search query.
	if m.ViaBot != nil {
		// Only clean up the prompt when this message was sent via this
		// bot's own inline suggestions, never for another bot.
		if b.botID > 0 && m.ViaBot.ID == b.botID {
			if err := b.pauseClientForms(m.From.ID); err != nil {
				return err
			}
			b.clearSelectedSearchPrompt(ctx, m.Chat.ID)
		}
		return nil
	}
	if m.Location != nil {
		if err := b.pauseClientForms(m.From.ID); err != nil {
			return err
		}
		// Keep the native location-sharing button available for the next search.
		return b.showNear(ctx, m.Chat.ID, m.Location)
	}
	chatID := m.Chat.ID
	input := strings.TrimSpace(m.Text)
	if input == "" {
		return nil
	}
	// Keep personal navigation ahead of search and draft text handlers.
	cmd := strings.SplitN(strings.ToLower(input), " ", 2)[0]
	if at := strings.IndexByte(cmd, '@'); at >= 0 {
		cmd = cmd[:at]
	}
	switch {
	case cmd == "/version":
		return b.Send(ctx, chatID, "Локации · версия "+botVersion, markup([]Button{cabinetButton(), menuButton()}))
	case cmd == "/myrequests" || strings.EqualFold(input, "Мои заявки") || input == "📋 Мои заявки":
		defer b.observeVisitor(m.From, "Мои заявки", false)
		return b.showCabinet(ctx, chatID, m.From.ID)
	}
	// Track navigation labels, never store raw user-entered text or phone numbers.
	if cmd == "/start" {
		defer b.observeVisitor(m.From, "Главное меню", true)
	} else if strings.HasPrefix(input, "/") {
		switch cmd {
		case "/list":
			b.observeVisitor(m.From, "Список локаций", false)
		case "/favorites":
			b.observeVisitor(m.From, "Избранное", false)
		case "/near":
			b.observeVisitor(m.From, "Рядом со мной", false)
		case "/map":
			b.observeVisitor(m.From, "Карта локаций", false)
		case "/compare":
			b.observeVisitor(m.From, "Сравнение локаций", false)
		}
	} else {
		label := "Поиск локации"
		if b.leads != nil {
			if _, draft := b.leads.draft(m.From.ID); draft {
				label = "Форма заявки"
			}
		}
		switch input {
		case "📋 Список локаций", "📋 Список станций":
			label = "Список локаций"
		case "⭐ Избранное":
			label = "Избранное"
		case "📝 Оставить заявку":
			label = "Форма заявки"
		case "🏠 Главное меню":
			label = "Главное меню"
		}
		b.observeVisitor(m.From, label, false)
	}
	if input == "🏠 Главное меню" || input == "📋 Список локаций" || input == "📋 Список станций" || input == "🔎 Поиск локации" || input == "🔎 Поиск" || input == "📍 Рядом со мной" || input == "⭐ Избранное" || input == "🗺 Карта локаций" || input == "⚖️ Сравнить локации" {
		if err := b.pauseClientForms(m.From.ID); err != nil {
			return err
		}
	}
	switch input {
	case "📋 Список локаций", "📋 Список станций":
		return b.showPage(ctx, chatID, 0)
	case "🔎 Поиск локации", "🔎 Поиск":
		return b.requestStationSearch(ctx, chatID)
	case "🗺 Карта локаций", "🗺 Открыть карту", "🗺 Общая карта":
		return b.showMap(ctx, chatID)
	case "📍 Рядом со мной":
		return b.requestLocation(ctx, chatID)
	case "⭐ Избранное":
		return b.showFavorites(ctx, chatID, m.From.ID)
	case "⚖️ Сравнить локации", "⚖️ Сравнить станции":
		return b.startComparison(ctx, chatID, m.From.ID, 0)
	case "🏠 Главное меню":
		return b.welcome(ctx, chatID, m.From)
	case "📝 Оставить заявку":
		return b.openLead(ctx, chatID, m.From.ID, "menu", "")
	}
	if strings.HasPrefix(input, "/") {
		if err := b.pauseClientForms(m.From.ID); err != nil {
			return err
		}
	}
	switch cmd {
	case "/start", "/help":
		return b.welcome(ctx, chatID, m.From)
	case "/list":
		return b.showPage(ctx, chatID, 0)
	case "/favorites":
		return b.showFavorites(ctx, chatID, m.From.ID)
	case "/compare":
		return b.startComparison(ctx, chatID, m.From.ID, 0)
	case "/near":
		return b.requestLocation(ctx, chatID)
	case "/map":
		return b.showMap(ctx, chatID)
	case "/id":
		return b.Send(ctx, chatID, fmt.Sprintf("Ваш Telegram ID: <code>%d</code>", m.From.ID), nil)
	case "/admin":
		if !b.isAdmin(m.From.ID) {
			return b.Send(ctx, chatID, "Нет доступа к админ-панели.", nil)
		}
		return b.showAdminMenu(ctx, chatID)
	case "/applications":
		if !b.adminIDs[m.From.ID] {
			return b.Send(ctx, chatID, "Доступно только администратору.", nil)
		}
		return b.showAdminLeads(ctx, chatID)
	case "/reload":
		if !b.adminIDs[m.From.ID] {
			return b.Send(ctx, chatID, "Доступно только администратору.", nil)
		}
		if err := b.store.Reload(); err != nil {
			return b.Send(ctx, chatID, "Ошибка конфигурации: "+html.EscapeString(err.Error()), nil)
		}
		return b.Send(ctx, chatID, "Данные из overrides.json обновлены.", nil)
	case "/sync":
		if !b.adminIDs[m.From.ID] {
			return b.Send(ctx, chatID, "Доступно только администратору. ID: /id", nil)
		}
		return b.startManualSync(ctx, chatID)
	case "/status":
		if !b.adminIDs[m.From.ID] {
			return b.Send(ctx, chatID, "Доступно только администратору.", nil)
		}
		items := b.store.List()
		pics, coords, ratings := 0, 0, 0
		for _, l := range items {
			if l.ImageURL != "" {
				pics++
			}
			if l.Latitude != nil && l.Longitude != nil {
				coords++
			}
			if l.Rating != "" {
				ratings++
			}
		}
		return b.Send(ctx, chatID, fmt.Sprintf("Локаций: %d\nФотографий: %d\nС координатами: %d\nС оценками: %d", len(items), pics, coords, ratings), nil)
	}
	if strings.HasPrefix(cmd, "/") {
		return b.Send(ctx, chatID, "Неизвестная команда. /help", nil)
	}
	if handled, err := b.handleAdminText(ctx, chatID, m.From, input); handled {
		return err
	}
	if handled, err := b.handleSupplementText(ctx, chatID, m.From, input); handled {
		return err
	}
	if handled, err := b.handleLeadText(ctx, chatID, m.From, input); handled {
		return err
	}
	if mapInput, ok := sharedMapInput(input); ok {
		point, err := resolveSharedMapPoint(ctx, b.http, mapInput)
		if err != nil {
			return b.Send(ctx, chatID, "📍 <b>Не получилось определить точку по ссылке.</b>\n\nВ Яндекс Картах откройте нужное место и скопируйте его <b>координаты</b>. Например: <code>55.751, 37.618</code>.\n\nОтправьте их сюда, и я покажу ближайшие локации.", markup(
				[]Button{urlButton("🗺 Открыть Яндекс Карты", "https://yandex.ru/maps/")},
				[]Button{callbackButton("📍 Как выбрать точку", "near"), menuButton()},
			))
		}
		return b.showNear(ctx, chatID, point)
	}
	found := catalog.Search(b.store.List(), input)
	if len(found) == 0 {
		return b.Send(ctx, chatID, "По запросу <b>"+html.EscapeString(input)+"</b> ничего не найдено. Попробуй часть названия.", nil)
	}
	if len(found) == 1 {
		err := b.showLocation(ctx, chatID, m.From.ID, found[0].ID, 0)
		if err == nil {
			b.recordLocationAnalytics(m.From.ID, found[0].ID, analytics.View)
		}
		return err
	}
	found = sortedStations(found)
	rows := [][]Button{}
	for i := 0; i < len(found) && i < 14; i += 2 {
		row := []Button{callbackButton(buttonLabel(found[i].Title), "loc:"+found[i].ID)}
		if i+1 < len(found) && i+1 < 14 {
			row = append(row, callbackButton(buttonLabel(found[i+1].Title), "loc:"+found[i+1].ID))
		}
		rows = append(rows, row)
	}
	rows = append(rows, []Button{callbackButton("⬅️ К списку", "page:0"), menuButton()})
	return b.Send(ctx, chatID, fmt.Sprintf("Найдено: %d. Выберите локацию:", len(found)), markup(rows...))
}
func (b *Bot) showMap(ctx context.Context, chatID int64) error {
	return b.Send(ctx, chatID, "<b>Карта локаций</b> 🗺\n\nПосмотрите, где расположены наши локации. Откройте карту, чтобы изучить выбранный район, дороги и ближайшие ориентиры.", markup([]Button{urlButton("Открыть карту", b.mapURL)}, []Button{callbackButton("⬅️ К списку", "page:0"), menuButton()}))
}
func (b *Bot) requestLocation(ctx context.Context, chatID int64) error {
	text := "📍 <b>Локации рядом с Вами</b>\n\nУкажите точку, которая Вас интересует, и мы покажем до пяти ближайших локаций.\n\n" +
		"<b>С компьютера:</b>\n" +
		"1. Откройте Яндекс Карты.\n" +
		"2. Выберите место на карте → <b>«Что здесь?»</b> → <b>«Поделиться»</b> → <b>«Скопировать ссылку»</b>.\n" +
		"3. Отправьте ссылку сюда в чат.\n\n" +
		"Если ссылка не сработает, пришлите <b>координаты точки</b>, например: <code>55.751, 37.618</code>.\n\n" +
		"<b>С телефона:</b> отправьте геопозицию через 📎."
	return b.sendInlineAndRemoveReplyKeyboard(ctx, chatID, text, markup(
		[]Button{urlButton("🗺 Открыть Яндекс Карты", "https://yandex.ru/maps/")},
		[]Button{callbackButton("⬅️ К списку локаций", "page:0"), menuButton()},
	))
}
func (b *Bot) Run(ctx context.Context) error {
	me, err := b.call(ctx, "getMe", map[string]any{})
	if err != nil {
		return fmt.Errorf("Bot token verification: %w", err)
	}
	var identity user
	if err := json.Unmarshal(me, &identity); err == nil {
		b.botID = identity.ID
		b.botIdentityMu.Lock(); b.botUsername=identity.Username; b.botIdentityMu.Unlock()
	}
	nextExpiry := b.expireDueCases()
	go b.runCaseExpiry(ctx, nextExpiry)
	go b.runLeadDelivery(ctx)
	go b.runReferralNotices(ctx)
	go b.runKeyboardCleanup(ctx)
	go b.configureTelegramMenu(ctx)
	go b.autoBackupLoop(ctx)
	go b.visitorMonitor(ctx)
	var offset int64
	if raw, err := os.ReadFile(b.offsetFile); err == nil {
		offset, _ = strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	}
	log.Printf("Karty %s: Telegram long polling started (@%s)", botVersion, identity.Username)
	for ctx.Err() == nil {
		raw, err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 30, "limit": 30, "allowed_updates": []string{"message", "callback_query", "inline_query", "chosen_inline_result"}})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("getUpdates: %v", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(4 * time.Second):
				continue
			}
		}
		var updates []update
		if err = json.Unmarshal(raw, &updates); err != nil {
			log.Printf("Invalid getUpdates: %v", err)
			continue
		}
		for _, u := range updates {
			if err = b.handle(ctx, u); err != nil {
				log.Printf("Update %d: %v", u.UpdateID, err)
			}
			offset = u.UpdateID + 1
			if err = os.WriteFile(b.offsetFile, []byte(strconv.FormatInt(offset, 10)), 0600); err != nil {
				log.Printf("Update offset save: %v", err)
			}
		}
	}
	return nil
}
