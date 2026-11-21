package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type referralTransport func(*http.Request) (*http.Response, error)

func (f referralTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReferralDecisionDelivery(t *testing.T) {
	t.Setenv("MINIAPP_URL", "https://example.test/app?lang=ru")
	store, err := openLeadStore(filepath.Join(t.TempDir(), "leads.json"))
	if err != nil {
		t.Fatal(err)
	}
	bot := &Bot{leads: store, adminIDs: map[int64]bool{90: true}}
	code, err := store.newReferralCode(101)
	if err != nil {
		t.Fatal(err)
	}
	draft := leadDraft{Kind: "existing", Name: "Test", Phone: "+79990000000", LocationID: "001"}
	lead, _, err := store.submitMiniCase(202, "test", "Метро", draft, code, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.takeInWork(lead.Number, 90); err != nil {
		t.Fatal(err)
	}

	type notice struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
		Markup struct {
			Rows [][]struct {
				Text   string `json:"text"`
				WebApp struct {
					URL string `json:"url"`
				} `json:"web_app"`
			} `json:"inline_keyboard"`
		} `json:"reply_markup"`
	}
	var notices []notice
	fail := false
	bot.http = &http.Client{Transport: referralTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/bot/sendMessage" {
			t.Fatalf("unexpected method %s", r.URL.Path)
		}
		var n notice
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			t.Fatal(err)
		}
		if fail {
			return nil, errors.New("temporary delivery failure")
		}
		notices = append(notices, n)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`))}, nil
	})}
	ctx := context.Background()
	rec, changed, err := bot.MiniAdminReferralAction(90, lead.Number, "defer")
	if err != nil || !changed {
		t.Fatalf("defer: %v", err)
	}
	bot.MiniDeferredReminder(ctx, 90, rec)
	bot.sendReferralNotices(ctx)
	if len(notices) != 1 || notices[0].ChatID != 90 {
		t.Fatalf("defer recipients: %+v", notices)
	}
	_, changed, err = bot.MiniAdminReferralAction(90, lead.Number, "award")
	if err != nil || !changed {
		t.Fatalf("award: %v", err)
	}
	fail = true
	bot.sendReferralNotices(ctx)
	if len(store.referralNotices()) != 1 {
		t.Fatal("failed notification was lost")
	}
	fail = false
	bot.sendReferralNotices(ctx)
	bot.sendReferralNotices(ctx)
	if len(notices) != 2 || notices[1].ChatID != 101 || !strings.Contains(notices[1].Text, "10%") {
		t.Fatalf("award recipients: %+v", notices)
	}
	rows := notices[1].Markup.Rows
	if len(rows) != 1 || len(rows[0]) != 1 || !strings.Contains(rows[0][0].Text, "Использовать скидку") || rows[0][0].WebApp.URL != "https://example.test/app?lang=ru&view=profile" {
		t.Fatalf("unexpected profile button: %+v", rows)
	}
}

func TestConcurrentReferralChoiceBelongsToWinningCase(t *testing.T) {
	s, err := openLeadStore(filepath.Join(t.TempDir(), "leads.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, err := s.newReferralCode(1)
	if err != nil {
		t.Fatal(err)
	}
	for invitee := int64(10); invitee < 30; invitee++ {
		start := make(chan struct{})
		type result struct {
			lead    leadApplication
			already bool
			err     error
		}
		out := make(chan result, 2)
		for _, skip := range []bool{false, true} {
			go func(skip bool) {
				name := "Invited"
				if skip {
					name = "Opted out"
				}
				d := leadDraft{Kind: "existing", Name: name, Phone: "+79990000000", LocationID: "001"}
				<-start
				lead, already, err := s.submitMiniCase(invitee, "", "Метро", d, code, skip)
				out <- result{lead, already, err}
			}(skip)
		}
		close(start)
		first, second := <-out, <-out
		if first.err != nil || second.err != nil || first.already == second.already || first.lead.Number != second.lead.Number {
			t.Fatalf("different commits: %+v, %+v", first, second)
		}
		if (first.lead.Name == "Invited") != (first.lead.ReferrerID == 1) {
			t.Fatalf("choice belongs to another request: %+v", first.lead)
		}
	}
}
