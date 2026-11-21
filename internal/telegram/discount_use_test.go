package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDiscountApprovalsShareOneDurableGrant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leads.json")
	s, err := openLeadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{leads: s, adminIDs: map[int64]bool{90: true, 91: true}}
	next := s.cloneLocked()
	next.Profiles["101"] = MiniProfile{UserID: 101, Name: "Ирина", DiscountPercent: 15}
	for i, key := range []string{"201", "202"} {
		next.Referrals[key] = referralRecord{InviterID: 101, InviteeID: int64(i + 201), CaseNumber: int64(i + 1), Status: "awarded", Percent: 10}
	}
	if err := s.saveLocked(next); err != nil {
		t.Fatal(err)
	}
	w := b.MiniDiscountWallet(101)
	if w.Percent != 15 {
		t.Fatal(w)
	}
	if _, err := b.MiniRequestDiscount(102, w.Offer); err == nil {
		t.Fatal("another user spent this grant")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state, err := b.MiniRequestDiscount(101, w.Offer)
			if err != nil || len(state.History) != 1 {
				t.Errorf("duplicate request: %+v %v", state, err)
			}
		}()
	}
	wg.Wait()
	if _, err := b.MiniAdminSetDiscount(90, 101, 5); err == nil {
		t.Fatal("reserved manual discount changed")
	}
	if _, err := b.MiniApproveDiscount(101, 1); err == nil {
		t.Fatal("client approved own discount")
	}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.MiniApproveDiscount(90, 1)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	b.leads, err = openLeadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := b.MiniRequestDiscount(101, w.Offer)
	if err != nil || len(current.History) != 1 || current.Percent != 10 || current.History[0].Status != "used" {
		t.Fatalf("stale replay / restart: %+v %v", current, err)
	}
	for id := int64(2); id <= 3; id++ {
		current = b.MiniDiscountWallet(101)
		if current.Percent != 10 {
			t.Fatalf("lost separate referral: %+v", current)
		}
		_, err = b.MiniRequestDiscount(101, current.Offer)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = b.MiniApproveDiscount(91, id); err != nil {
			t.Fatal(err)
		}
	}
	if p, ok := b.MiniGetProfile(101); !ok || p.DiscountPercent != 0 {
		t.Fatalf("spent grant returned: %+v", p)
	}
	p, err := b.MiniAdminSetDiscount(90, 101, 15)
	if err != nil || p.DiscountPercent != 15 {
		t.Fatal(err, p)
	}
	if newWallet := b.MiniDiscountWallet(101); newWallet.Offer == w.Offer {
		t.Fatal("manual re-award reused previous offer")
	}
}
func TestDiscountTelegramApprovalAndRetry(t *testing.T) {
	t.Setenv("MINIAPP_URL", "https://example.test/mini")
	path := filepath.Join(t.TempDir(), "leads.json")
	s, err := openLeadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{leads: s, adminIDs: map[int64]bool{90: true, 91: true}}
	next := s.cloneLocked()
	next.Profiles["7"] = MiniProfile{UserID: 7, Name: "Саша", DiscountPercent: 10}
	if err = s.saveLocked(next); err != nil {
		t.Fatal(err)
	}
	w, err := b.MiniRequestDiscount(7, b.MiniDiscountWallet(7).Offer)
	if err != nil || w.History[0].Status != "pending" {
		t.Fatal(w, err)
	}
	var sent, edited = map[int64]int{}, map[int64]int{}
	fail := true
	b.http = &http.Client{Transport: referralTransport(func(r *http.Request) (*http.Response, error) {
		var p struct {
			Chat   int64           `json:"chat_id"`
			Text   string          `json:"text"`
			Markup json.RawMessage `json:"reply_markup"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Fatal(err)
		}
		switch filepath.Base(r.URL.Path) {
		case "sendMessage":
			if p.Chat == 7 {
				if fail {
					return nil, errors.New("temporary failure")
				}
				if !strings.Contains(p.Text, "10% использована") || !strings.Contains(string(p.Markup), "?view=discount") {
					t.Fatal(p.Text, string(p.Markup))
				}
			} else if !strings.Contains(string(p.Markup), "discount:1") {
				t.Fatal(string(p.Markup))
			}
			sent[p.Chat]++
		case "editMessageText":
			edited[p.Chat]++
		case "answerCallbackQuery":
		default:
			t.Fatalf("unexpected method %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":4}}`))}, nil
	})}
	ctx := context.Background()
	b.sendDiscountNotices(ctx)
	b.sendDiscountNotices(ctx)
	cb := &callbackQuery{ID: "t", From: user{ID: 7}, Message: &message{Chat: chat{Type: "private", ID: 7}}, Data: "discount:1"}
	if err = b.handleDiscountCallback(ctx, cb); err != nil {
		t.Fatal(err)
	}
	if b.MiniDiscountWallet(7).History[0].Status != "pending" {
		t.Fatal("client callback accepted")
	}
	cb.From.ID = 90
	cb.Message.Chat.ID = 90
	if err = b.handleDiscountCallback(ctx, cb); err != nil {
		t.Fatal(err)
	}
	if _, err = b.MiniApproveDiscount(91, 1); err != nil {
		t.Fatal(err)
	}
	b.sendDiscountNotices(ctx)
	b.leads, err = openLeadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	fail = false
	b.sendDiscountNotices(ctx)
	b.sendDiscountNotices(ctx)
	if sent[90] != 1 || sent[91] != 1 || sent[7] != 1 || edited[90] != 1 || edited[91] != 1 {
		t.Fatalf("sent %v, edited %v", sent, edited)
	}
}
