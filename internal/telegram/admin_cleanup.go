package telegram

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"locationbot/internal/adminbackup"
)

const cleanupPageSize = 8

type cleanupRecord struct {
	id    int64
	label string
}

func (b *Bot) beginCleanup(ctx context.Context, chatID, adminID int64, kind string, trash bool) error {
	if kind != "l" && kind != "v" {
		return b.showAdminMenu(ctx, chatID)
	}
	session := b.adminSession(adminID)
	session.CleanupKind = kind
	session.CleanupTrash = trash
	session.CleanupPage = 0
	session.CleanupSelected = map[int64]bool{}
	session.CleanupAction = ""
	return b.showAdminCleanup(ctx, chatID, session)
}
func (b *Bot) cleanupItems(session *adminSession) []cleanupRecord {
	rows := []cleanupRecord{}
	if session.CleanupKind == "l" {
		if b.leads == nil {
			return rows
		}
		for _, lead := range b.leads.allSubmitted() {
			if !lead.isResolved() || lead.Hidden != session.CleanupTrash {
				continue
			}
			name := strings.TrimSpace(lead.Name)
			rows = append(rows, cleanupRecord{id: lead.Number, label: fmt.Sprintf("№%04d · %s", lead.Number, limitRunes(name, 25))})
		}
	} else if session.CleanupKind == "v" {
		if b.visitors == nil {
			return rows
		}
		for _, p := range b.visitors.snapshot() {
			if p.Hidden != session.CleanupTrash {
				continue
			}
			name := strings.TrimSpace(p.FirstName + " " + p.LastName)
			if name == "" {
				name = fmt.Sprintf("ID %d", p.UserID)
			}
			rows = append(rows, cleanupRecord{id: p.UserID, label: limitRunes(name, 28)})
		}
	}
	return rows
}
func (b *Bot) showAdminCleanup(ctx context.Context, chatID int64, s *adminSession) error {
	if s.CleanupKind != "l" && s.CleanupKind != "v" {
		return b.showAdminMenu(ctx, chatID)
	}
	items := b.cleanupItems(s)
	pages := max(1, (len(items)+cleanupPageSize-1)/cleanupPageSize)
	if s.CleanupPage < 0 {
		s.CleanupPage = 0
	}
	if s.CleanupPage >= pages {
		s.CleanupPage = pages - 1
	}
	kindName := "Заявки"
	back := "adm:leads"
	if s.CleanupKind == "v" {
		kindName = "Посетители"
		back = "adm:visitors"
	}
	section := "Активные"
	if s.CleanupTrash {
		section = "Корзина"
	}
	rows := [][]Button{
		{callbackButton("📋 Активные", "adm:clean:start:"+s.CleanupKind+":a"), callbackButton("♻️ Корзина", "adm:clean:start:"+s.CleanupKind+":t")},
	}
	for _, record := range items[s.CleanupPage*cleanupPageSize : min((s.CleanupPage+1)*cleanupPageSize, len(items))] {
		marker := "☐"
		if s.CleanupSelected[record.id] {
			marker = "✅"
		}
		rows = append(rows, []Button{callbackButton(marker+" "+record.label, fmt.Sprintf("adm:clean:toggle:%d", record.id))})
	}
	nav := []Button{}
	if s.CleanupPage > 0 {
		nav = append(nav, callbackButton("◀️ Назад", fmt.Sprintf("adm:clean:page:%d", s.CleanupPage-1)))
	}
	if s.CleanupPage+1 < pages {
		nav = append(nav, callbackButton("Далее ▶️", fmt.Sprintf("adm:clean:page:%d", s.CleanupPage+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []Button{callbackButton("✅ Выбрать страницу", "adm:clean:all"), callbackButton("✖️ Снять выбор", "adm:clean:clear")})
	if s.CleanupTrash {
		rows = append(rows, []Button{callbackButton("♻️ Восстановить выбранные", "adm:clean:ask:r")})
		rows = append(rows, []Button{callbackButton("🗑 Удалить навсегда", "adm:clean:ask:e")})
	} else {
		rows = append(rows, []Button{callbackButton("🗑 Скрыть выбранные", "adm:clean:ask:h")})
	}
	rows = append(rows, []Button{callbackButton("⬅️ Назад", back)})
	msg := fmt.Sprintf("🗑 <b>Управление данными · %s</b>\n\nРаздел: <b>%s</b>\nВсего: %d · Выбрано: %d\nСтраница %d/%d\n\nНажимайте на записи для выбора. Можно выбрать несколько записей на разных страницах.",
		kindName, section, len(items), len(s.CleanupSelected), s.CleanupPage+1, pages)
	if s.CleanupKind == "l" && !s.CleanupTrash {
		msg += "\n\nАктивные обращения нельзя скрыть или удалить. Сначала завершите обращение."
	}
	if s.CleanupTrash {
		msg += "\n\nВосстановление вернёт запись в список. Окончательное удаление уберёт её с сервера, а не из уже доставленных сообщений Telegram."
	}
	return b.Send(ctx, chatID, msg, markup(rows...))
}
func (b *Bot) cleanupSelection(s *adminSession) ([]int64, error) {
	items := b.cleanupItems(s)
	valid := map[int64]bool{}
	for _, item := range items {
		valid[item.id] = true
	}
	ids := make([]int64, 0, len(s.CleanupSelected))
	for id := range s.CleanupSelected {
		if !valid[id] {
			return nil, fmt.Errorf("запись %d изменилась или недоступна, обновите список", id)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("сначала выберите хотя бы одну запись")
	}
	// Stable order for logs and consistent multi-row operations.
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	if len(ids) > 100 {
		return nil, fmt.Errorf("за одну операцию можно обработать не более 100 записей")
	}
	return ids, nil
}
func (b *Bot) showCleanupConfirm(ctx context.Context, chatID int64, s *adminSession, action string) error {
	if action != "h" && action != "r" && action != "e" {
		return b.showAdminCleanup(ctx, chatID, s)
	}
	if (action == "h") == s.CleanupTrash {
		return b.showAdminCleanup(ctx, chatID, s)
	}
	ids, err := b.cleanupSelection(s)
	if err != nil {
		return b.Send(ctx, chatID, "⚠️ "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К выбору", "adm:clean:back")}))
	}
	s.CleanupAction = action
	title := "Скрыть записи"
	if action == "r" {
		title = "Восстановить записи"
	}
	if action == "e" {
		title = "Окончательно удалить записи"
	}
	warning := ""
	if action == "e" {
		warning = "\n\n⚠️ Действие необратимо. Старые ZIP-бэкапы не будут повторно добавлять удалённые записи. В резервных архивах, созданных до удаления, персональные данные физически останутся до уничтожения этих архивов."
	}
	msg := fmt.Sprintf("⚠️ <b>%s?</b>\n\nВыбрано записей: <b>%d</b>\nID: <code>%s</code>%s", title, len(ids), html.EscapeString(formatCleanupIDs(ids)), warning)
	return b.Send(ctx, chatID, msg, markup(
		[]Button{callbackButton("✅ Подтвердить", "adm:clean:apply:"+action)},
		[]Button{callbackButton("❌ Отмена", "adm:clean:back")},
	))
}
func formatCleanupIDs(ids []int64) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return strings.Join(out, ", ")
}
func (b *Bot) applyCleanup(ctx context.Context, chatID int64, s *adminSession, action string) error {
	if action != s.CleanupAction || action == "" {
		return b.showAdminCleanup(ctx, chatID, s)
	}
	b.backupMu.Lock()
	ids, err := b.cleanupSelection(s)
	if err == nil {
		if s.CleanupKind == "l" {
			if b.leads == nil {
				err = fmt.Errorf("хранилище заявок недоступно")
			} else {
				switch action {
				case "h":
					err = b.leads.setHidden(ids, true)
				case "r":
					err = b.leads.setHidden(ids, false)
				case "e":
					// Mark first. A failed erase keeps the record in the trash, while a
					// successful mark prevents older backups from resurrecting its PII.
					err = adminbackup.MarkDeleted(b.store.DataDir(), ids, nil)
					if err == nil {
						err = b.leads.eraseHidden(ids)
					}
				}
			}
		} else if s.CleanupKind == "v" {
			if b.visitors == nil {
				err = fmt.Errorf("хранилище посетителей недоступно")
			} else {
				switch action {
				case "h":
					err = b.visitors.setHidden(ids, true)
				case "r":
					err = b.visitors.setHidden(ids, false)
				case "e":
					err = adminbackup.MarkDeleted(b.store.DataDir(), nil, ids)
					if err == nil {
						err = b.visitors.eraseHidden(ids)
					}
				}
			}
		} else {
			err = fmt.Errorf("неизвестный раздел")
		}
	}
	b.backupMu.Unlock()
	s.CleanupAction = ""
	if err != nil {
		log.Printf("admin cleanup operation %s: %v", action, err)
		return b.Send(ctx, chatID, "❌ Не удалось выполнить действие: "+html.EscapeString(err.Error()), markup([]Button{callbackButton("⬅️ К выбору", "adm:clean:back")}))
	}
	s.CleanupSelected = map[int64]bool{}
	message := "✅ Изменения сохранены."
	if action == "h" {
		message = "✅ Записи перемещены в корзину. Их можно восстановить."
	}
	if action == "r" {
		message = "✅ Записи восстановлены."
	}
	if action == "e" {
		message = "✅ Записи окончательно удалены из рабочего хранилища. Служебные отметки об удалении сохранены для защиты от восстановления из старых бэкапов."
	}
	return b.Send(ctx, chatID, message, markup([]Button{callbackButton("⬅️ К списку", "adm:clean:back")}))
}
func (b *Bot) handleAdminCleanup(ctx context.Context, chatID, userID int64, data string) (bool, error) {
	if !strings.HasPrefix(data, "adm:clean:") {
		return false, nil
	}
	s := b.adminSession(userID)
	parts := strings.Split(data, ":")
	if len(parts) < 3 {
		return true, b.showAdminMenu(ctx, chatID)
	}
	switch parts[2] {
	case "start":
		if len(parts) != 5 {
			return true, b.showAdminMenu(ctx, chatID)
		}
		return true, b.beginCleanup(ctx, chatID, userID, parts[3], parts[4] == "t")
	case "one":
		if len(parts) != 5 {
			return true, b.showAdminMenu(ctx, chatID)
		}
		id, err := strconv.ParseInt(parts[4], 10, 64)
		if err != nil || id <= 0 {
			return true, b.showAdminMenu(ctx, chatID)
		}
		if err := b.beginCleanup(ctx, chatID, userID, parts[3], false); err != nil {
			return true, err
		}
		// Do not select a pending application.
		valid := false
		for _, rec := range b.cleanupItems(s) {
			if rec.id == id {
				valid = true
				break
			}
		}
		if !valid {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		s.CleanupSelected[id] = true
		return true, b.showCleanupConfirm(ctx, chatID, s, "h")
	case "back":
		return true, b.showAdminCleanup(ctx, chatID, s)
	case "page":
		if len(parts) != 4 {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		p, err := strconv.Atoi(parts[3])
		if err == nil {
			s.CleanupPage = p
		}
		return true, b.showAdminCleanup(ctx, chatID, s)
	case "toggle":
		if len(parts) != 4 {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		id, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		valid := false
		for _, rec := range b.cleanupItems(s) {
			if rec.id == id {
				valid = true
				break
			}
		}
		if !valid {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		if s.CleanupSelected == nil {
			s.CleanupSelected = map[int64]bool{}
		}
		if s.CleanupSelected[id] {
			delete(s.CleanupSelected, id)
		} else {
			s.CleanupSelected[id] = true
		}
		s.CleanupAction = ""
		return true, b.showAdminCleanup(ctx, chatID, s)
	case "all":
		if s.CleanupSelected == nil {
			s.CleanupSelected = map[int64]bool{}
		}
		items := b.cleanupItems(s)
		from := max(0, s.CleanupPage*cleanupPageSize)
		for _, record := range items[from:min(from+cleanupPageSize, len(items))] {
			s.CleanupSelected[record.id] = true
		}
		s.CleanupAction = ""
		return true, b.showAdminCleanup(ctx, chatID, s)
	case "clear":
		s.CleanupSelected = map[int64]bool{}
		s.CleanupAction = ""
		return true, b.showAdminCleanup(ctx, chatID, s)
	case "ask":
		if len(parts) != 4 {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		return true, b.showCleanupConfirm(ctx, chatID, s, parts[3])
	case "apply":
		if len(parts) != 4 {
			return true, b.showAdminCleanup(ctx, chatID, s)
		}
		return true, b.applyCleanup(ctx, chatID, s, parts[3])
	}
	return true, b.showAdminCleanup(ctx, chatID, s)
}

// Ledger protects permanently erased profiles against re-created alerts
// after another /start or restoring an old data snapshot.
func (b *Bot) isPermanentlyDeletedVisitor(userID int64) bool {
	if b == nil || b.store == nil || userID <= 0 {
		return false
	}
	ledger, err := adminbackup.ReadDeletionLedger(b.store.DataDir())
	if err != nil {
		log.Printf("cannot check erased visitor ledger: %v", err)
		// Fail closed: do not create new tracking records or alarms when the
		// suppression ledger cannot be read.
		return true
	}
	_, deleted := ledger.Visitors[strconv.FormatInt(userID, 10)]
	return deleted
}
func (b *Bot) cleanupStamp() time.Time { return time.Now().UTC() }
