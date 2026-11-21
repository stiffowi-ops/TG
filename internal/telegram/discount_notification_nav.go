package telegram

import (
 "context"
 "net/url"
)

// Open the actual rewards screen, not the profile landing screen.
// Works for both Telegram WebApp buttons and startapp deeplinks.
func (b *Bot) discountButton(ctx context.Context, label string) Button {
 button:=b.profileButton(ctx)
 if len(button)==0{return button}
 button["text"]=label
 if app,ok:=button["web_app"].(map[string]string);ok {
  if target,err:=url.Parse(app["url"]);err==nil {
   q:=target.Query()
   q.Set("view","discount")
   target.RawQuery=q.Encode()
   button["web_app"]=map[string]string{"url":target.String()}
  }
 }else if raw,ok:=button["url"].(string);ok {
  if target,err:=url.Parse(raw);err==nil {
   q:=target.Query()
   q.Set("startapp","discount")
   target.RawQuery=q.Encode()
   button["url"]=target.String()
  }
 }
 return button
}
