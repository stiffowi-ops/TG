package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Public receipt. Delivery information and the grant being spent stay server-side.
type MiniDiscountUse struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"userId"`
	Name       string     `json:"name"`
	Percent    int        `json:"percent"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ApprovedAt *time.Time `json:"approvedAt,omitempty"`
}
type discountUseRecord struct {
	MiniDiscountUse
	Source         string          `json:"source"`
	Grant          string          `json:"grant"`
	ApprovedBy     int64           `json:"approvedBy,omitempty"`
	AdminMessages  map[int64]int64 `json:"adminMessages,omitempty"` // negative once updated
	ClientNotified bool            `json:"clientNotified,omitempty"`
	ClientRequestNotified bool     `json:"clientRequestNotified,omitempty"`
}
type MiniDiscountWallet struct {
	Percent int               `json:"percent"`
	Offer   string            `json:"offer,omitempty"`
	History []MiniDiscountUse `json:"history"`
}

func (s *leadStore) discountOfferLocked(id int64) (int, string, string) {
	p, _ := s.profileLocked(id)
	var best referralRecord
	for _, r := range s.state.Referrals {
		if r.InviterID != id || r.Status != "awarded" || r.UsedByRequest != 0 || r.Percent <= 0 {
			continue
		}
		if r.Percent > best.Percent || (r.Percent == best.Percent && r.CaseNumber < best.CaseNumber) {
			best = r
		}
	}
	if best.Percent > 0 && best.Percent >= p.DiscountPercent {
		return best.Percent, "referral", fmt.Sprintf("r%d", best.CaseNumber)
	}
	if p.DiscountPercent > 0 {
		return p.DiscountPercent, "manual", fmt.Sprintf("m%d", p.ManualRevision)
	}
	return 0, "", ""
}
func (s *leadStore) discountRowsLocked(userID int64) []MiniDiscountUse {
	rows := []MiniDiscountUse{}
	for _, r := range s.state.DiscountUses {
		if userID == 0 || r.UserID == userID {
			rows = append(rows, r.MiniDiscountUse)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID > rows[j].ID })
	return rows
}
func (s *leadStore) walletLocked(id int64) MiniDiscountWallet {
	percent, _, grant := s.discountOfferLocked(id)
	return MiniDiscountWallet{Percent: percent, Offer: grant, History: s.discountRowsLocked(id)}
}
func (b *Bot) MiniDiscountWallet(id int64) MiniDiscountWallet {
	if b == nil || b.leads == nil || id <= 0 {
		return MiniDiscountWallet{}
	}
	s := b.leads
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.walletLocked(id)
}
func (b *Bot) wakeDiscountDelivery() {
	select {
	case b.referralWake <- struct{}{}:
	default:
	}
}
func (b *Bot) MiniRequestDiscount(userID int64, offer string) (MiniDiscountWallet, error) {
	if b == nil || b.leads == nil || userID <= 0 {
		return MiniDiscountWallet{}, errors.New("доступ запрещён")
	}
	s := b.leads
	s.mu.Lock()
	defer s.mu.Unlock()
	var maxID int64
	for id, r := range s.state.DiscountUses {
		if r.UserID == userID && (r.Status == "pending" || r.Grant == offer) {
			return s.walletLocked(userID), nil
		}
		if id > maxID {
			maxID = id
		}
	}
	percent, source, grant := s.discountOfferLocked(userID)
	if percent == 0 || grant != offer {
		return MiniDiscountWallet{}, errors.New("доступная скидка изменилась. Обновите профиль")
	}
	rec := discountUseRecord{MiniDiscountUse: MiniDiscountUse{ID: maxID + 1, UserID: userID, Name: s.referralDisplayNameLocked(userID), Percent: percent, Status: "pending", CreatedAt: time.Now().UTC()}, Source: source, Grant: grant}
	rec.AdminMessages = make(map[int64]int64)
	for admin, enabled := range b.adminIDs {
		if enabled {
			rec.AdminMessages[admin] = 0
		}
	}
	next := s.cloneLocked()
	next.DiscountUses[rec.ID] = rec
	if err := s.saveLocked(next); err != nil {
		return MiniDiscountWallet{}, err
	}
	b.wakeDiscountDelivery()
	return s.walletLocked(userID), nil
}

// The navigation badge needs only the count and the newest pending request.
func (b *Bot) MiniPendingDiscountCount(adminID int64) (int, int64) {
	if !b.IsMiniAdmin(adminID) || b.leads == nil {
		return 0, 0
	}
	s := b.leads
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	var latest int64
	for id, r := range s.state.DiscountUses {
		if r.Status == "pending" {
			count++
			if id > latest {
				latest = id
			}
		}
	}
	return count, latest
}

func (b *Bot) MiniAdminDiscountUses(adminID int64) ([]MiniDiscountUse, error) {
	if !b.IsMiniAdmin(adminID) || b.leads == nil {
		return nil, errors.New("доступ запрещён")
	}
	s := b.leads
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.discountRowsLocked(0), nil
}
func (b *Bot) MiniApproveDiscount(adminID, requestID int64) (MiniDiscountUse, error) {
	if !b.IsMiniAdmin(adminID) || b.leads == nil {
		return MiniDiscountUse{}, errors.New("доступ запрещён")
	}
	s := b.leads
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.state.DiscountUses[requestID]
	if !ok {
		return MiniDiscountUse{}, errors.New("запрос не найден")
	}
	if rec.Status == "used" {
		return rec.MiniDiscountUse, nil
	}
	next := s.cloneLocked()
	if rec.Source == "manual" {
		p, ok := s.profileLocked(rec.UserID)
		if !ok || p.DiscountPercent != rec.Percent || fmt.Sprintf("m%d", p.ManualRevision) != rec.Grant {
			return MiniDiscountUse{}, errors.New("ручная скидка изменилась")
		}
		p.DiscountPercent = 0
		p.ManualRevision++
		p.UpdatedAt = time.Now().UTC()
		next.Profiles[strconv.FormatInt(rec.UserID, 10)] = p
	} else {
		number, err := strconv.ParseInt(strings.TrimPrefix(rec.Grant, "r"), 10, 64)
		if err != nil {
			return MiniDiscountUse{}, errors.New("начисление недоступно")
		}
		r, ok := s.referralByCaseLocked(number)
		key := strconv.FormatInt(r.InviteeID, 10)
		if !ok || r.InviterID != rec.UserID || r.Status != "awarded" || r.UsedByRequest != 0 {
			return MiniDiscountUse{}, errors.New("начисление недоступно")
		}
		r.UsedByRequest = rec.ID
		next.Referrals[key] = r
	}
	now := time.Now().UTC()
	rec.Status = "used"
	rec.ApprovedAt = &now
	rec.ApprovedBy = adminID
	next.DiscountUses[rec.ID] = rec
	if err := s.saveLocked(next); err != nil {
		return MiniDiscountUse{}, err
	}
	b.wakeDiscountDelivery()
	return rec.MiniDiscountUse, nil
}
func (b *Bot) handleDiscountCallback(ctx context.Context, cb *callbackQuery) error {
	if cb.Message == nil || cb.Message.Chat.Type != "private" || cb.From.ID != cb.Message.Chat.ID || !b.IsMiniAdmin(cb.From.ID) {
		b.answerCallback(ctx, cb.ID)
		return nil
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(cb.Data, "discount:"), 10, 64)
	if err == nil {
		_, err = b.MiniApproveDiscount(cb.From.ID, id)
	}
	message := "Согласовано. Скидка использована."
	if err != nil {
		message = err.Error()
	}
	_, err = b.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": cb.ID, "text": message})
	return err
}
func (s *leadStore) saveDiscountReceipt(id, chat, message int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cloneLocked()
	r, ok := next.DiscountUses[id]
	if !ok {
		return errors.New("запрос не найден")
	}
	if chat == 0 {
		r.ClientNotified = true
	} else {
		if r.AdminMessages == nil {
			r.AdminMessages = map[int64]int64{}
		}
		r.AdminMessages[chat] = message
	}
	next.DiscountUses[id] = r
	return s.saveLocked(next)
}
func (s *leadStore) markDiscountRequestNotified(id int64) error {
 s.mu.Lock()
 defer s.mu.Unlock()
 r,ok:=s.state.DiscountUses[id]
 if !ok {return errors.New("запрос не найден")}
 if r.ClientRequestNotified||r.Status!="pending" {return nil}
 next:=s.cloneLocked()
 r.ClientRequestNotified=true
 next.DiscountUses[id]=r
 return s.saveLocked(next)
}
func (s *leadStore) discountRequestPending(id int64) bool {
 s.mu.Lock()
 defer s.mu.Unlock()
 r,ok:=s.state.DiscountUses[id]
 return ok&&r.Status=="pending"&&!r.ClientRequestNotified
}

func (b *Bot) sendDiscountNotices(ctx context.Context) {
	if b == nil || b.leads == nil {
		return
	}
	s := b.leads
	s.mu.Lock()
	var rows []discountUseRecord
	for _, r := range s.state.DiscountUses {
		rows = append(rows, r)
	} // immutable snapshots; writes use cloneLocked
	s.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	for _, r := range rows {
		if ctx.Err() != nil {
			return
		}
        if r.Status == "pending" && !r.ClientRequestNotified && s.discountRequestPending(r.ID) {
            btn:=b.profileButton(ctx)
            if len(btn)>0 {
                btn["text"]="📱 Посмотреть статус скидки"
                _,err:=b.call(ctx,"sendMessage",map[string]any{
                    "chat_id":r.UserID,
                    "text":fmt.Sprintf("🕓 Запрос на использование скидки %d%% отправлен.\n\nОжидает подтверждения. Мы сообщим о результате.",r.Percent),
                    "reply_markup":markup([]Button{btn}),
                })
                if err==nil {err=s.markDiscountRequestNotified(r.ID)}
                if err!=nil {log.Printf("Discount request notice %d: %v",r.ID,err)}
            }
        }
		if r.Status == "used" && !r.ClientNotified {
			btn := b.profileButton(ctx)
			if len(btn) > 0 {
                btn["text"]="👤 Посмотреть скидку"
				_, err := b.call(ctx, "sendMessage", map[string]any{"chat_id": r.UserID, "text": fmt.Sprintf("✅ Ваша скидка %d%% использована. Применение скидки подтверждено.\n\nСпасибо! Будем рады новым обращениям.", r.Percent), "reply_markup": markup([]Button{btn})})
				if err == nil {
					err = s.saveDiscountReceipt(r.ID, 0, 0)
				}
				if err != nil {
					log.Printf("Discount confirmation %d: %v", r.ID, err)
				}
			}
		}
		for admin, mid := range r.AdminMessages {
			if !b.IsMiniAdmin(admin) || mid < 0 {
				continue
			}
			text := fmt.Sprintf("Согласовано · скидка %d%% использована\n%s · Telegram ID %d", r.Percent, r.Name, r.UserID)
			var err error
			if mid == 0 {
				payload := map[string]any{"chat_id": admin, "text": text}
				if r.Status == "pending" {
					payload["text"] = fmt.Sprintf("Запрос на использование скидки\n\n%s хочет использовать скидку %d%%.\nTelegram ID %d", r.Name, r.Percent, r.UserID)
					payload["reply_markup"] = markup([]Button{callbackButton("Согласовать", fmt.Sprintf("discount:%d", r.ID))})
				}
				var raw json.RawMessage
				raw, err = b.call(ctx, "sendMessage", payload)
				if err == nil {
					var reply struct {
						ID int64 `json:"message_id"`
					}
					err = json.Unmarshal(raw, &reply)
					if err == nil && reply.ID > 0 {
						receipt := reply.ID
						if r.Status == "used" {
							receipt = -receipt
						}
						err = s.saveDiscountReceipt(r.ID, admin, receipt)
					}
				}
			} else if r.Status == "used" {
				_, err = b.call(ctx, "editMessageText", map[string]any{"chat_id": admin, "message_id": mid, "text": text, "reply_markup": map[string]any{"inline_keyboard": [][]Button{}}})
				if err == nil || strings.Contains(fmt.Sprint(err), "message is not modified") || strings.Contains(fmt.Sprint(err), "message to edit not found") {
					err = s.saveDiscountReceipt(r.ID, admin, -mid)
				}
			}
			if err != nil {
				log.Printf("Discount request %d admin delivery: %v", r.ID, err)
			}
		}
	}
}
