package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// The test bot is fully separate from the existing bot. It only responds to
// /start with a Mini App launch button. It never reads or modifies real leads.
type client struct {
	token string
	http  *http.Client
}

func (c client) call(ctx context.Context, method string, input any, result any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+c.token+"/"+method, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if !envelope.OK {
		return errors.New(envelope.Description)
	}
	if result != nil {
		return json.Unmarshal(envelope.Result, result)
	}
	return nil
}
func launchButton(appURL string) map[string]any {
	return map[string]any{"inline_keyboard": [][]map[string]any{{{"text": "Открыть приложение", "web_app": map[string]any{"url": appURL}}}}}
}
func main() {
	token := strings.TrimSpace(os.Getenv("TEST_BOT_TOKEN"))
	appURL := strings.TrimSpace(os.Getenv("MINIAPP_URL"))
	u, e := url.Parse(appURL)
	if token == "" || e != nil || u.Scheme != "https" || u.Host == "" {
		log.Fatal("Set TEST_BOT_TOKEN and a public HTTPS MINIAPP_URL")
	}
	c := client{token: token, http: &http.Client{Timeout: 40 * time.Second}}
	ctx := context.Background()
	// Menu button is set only for this test token; never reuse production tokens.
	err := c.call(ctx, "setChatMenuButton", map[string]any{"menu_button": map[string]any{"type": "web_app", "text": "Открыть приложение", "web_app": map[string]any{"url": appURL}}}, nil)
	if err != nil {
		log.Printf("Test bot menu button: %v", err)
	}
	log.Println("Test bot is ready: send /start")
	var offset int64
	for {
		var updates []struct {
			UpdateID int64 `json:"update_id"`
			Message  *struct {
				Text string `json:"text"`
				Chat struct {
					ID   int64  `json:"id"`
					Type string `json:"type"`
				} `json:"chat"`
			} `json:"message"`
		}
		err := c.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 25, "allowed_updates": []string{"message"}}, &updates)
		if err != nil {
			log.Printf("Polling test bot: %v", err)
			time.Sleep(4 * time.Second)
			continue
		}
		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.Message == nil || update.Message.Chat.Type != "private" || !strings.HasPrefix(update.Message.Text, "/start") {
				continue
			}
			if err := c.call(ctx, "sendMessage", map[string]any{"chat_id": update.Message.Chat.ID, "text": "Откройте приложение, чтобы изучить локации, избранное и демонстрацию заявок.", "reply_markup": launchButton(appURL)}, nil); err != nil {
				log.Printf("Send test button: %s", fmt.Sprint(err))
			}
		}
	}
}
