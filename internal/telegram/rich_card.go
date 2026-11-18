package telegram

import (
 "context"
 "html"
 "log"
 "strconv"
 "strings"

 "locationbot/internal/catalog"
)

// Rich Messages render native, initially closed accordion sections.
// Every user-controlled field is HTML-escaped.
func richLocationHTML(raw catalog.Location, layerURL string) string {
 l := publicLocation(raw)
 title := html.EscapeString(limitCaptionUnits(l.Title, 150))
 icon := "📍"
 if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l.Title)), "метро ") {icon = "🚇"}
 status := strings.TrimSpace(l.LineStatus)
 if status == "" {status = "Статус пока не указан."}
 landmark := strings.TrimSpace(l.Landmarks)
 if landmark == "" {landmark = "Пока не указан."}
 traffic := l.Traffic
 if strings.TrimSpace(traffic) == "" && strings.TrimSpace(l.Description) != "" {traffic = l.Description}
 blocks := []struct {title, content string}{
  {"🚶 Трафик и локация",traffic},
  {"👥 Кто платит",l.Payers},
  {"📈 Возможности для бизнеса",l.Opportunity},
  {"🎯 Почему именно здесь",l.Conclusion},
 }
 if l.Sections != nil {
  blocks=nil
  for _,section:=range l.Sections {if section.Enabled {blocks=append(blocks,struct{title,content string}{section.Title,section.Body})}}
 }
 var buf strings.Builder
 buf.WriteString("<h3>" + icon + " " + title + "</h3>")
 buf.WriteString("<p><i>" + html.EscapeString(limitCaptionUnits(status,200)) + "</i></p>")
 buf.WriteString("<p>📍 <b>Ориентир:</b> " + html.EscapeString(limitCaptionUnits(landmark,310)) + "</p>")
 buf.WriteString("<hr/>")
 for _, b := range blocks {
  buf.WriteString("<details><summary>" + html.EscapeString(b.title) + "</summary><p>")
  buf.WriteString(html.EscapeString(cardField(b.content)))
  buf.WriteString("</p></details>")
 }
 if rating:=strings.TrimSpace(l.Rating);rating!=""{
  buf.WriteString("<p>⭐ <b>Оценка:</b> "+html.EscapeString(limitCaptionUnits(rating,100))+"</p>")
 }
 buf.WriteString("<hr/><p>🗺 <b>Локация на карте:</b> ")
 if link:=stationMapLink(l,layerURL);link!=""{
  buf.WriteString("<a href=\""+html.EscapeString(link)+"\">Открыть карту</a>")
 }else{
  buf.WriteString("Ссылка пока не настроена.")
 }
 buf.WriteString("</p>")
 return buf.String()
}

func (b *Bot) sendRichLocation(ctx context.Context, chatID int64, loc catalog.Location, keyboard any) error {
 rich := map[string]any{"html":richLocationHTML(loc,b.mapURL)}
 if inlineID, ok := inlineNavigationFromContext(ctx); ok {
  _, err := b.call(ctx,"editMessageText",map[string]any{
   "inline_message_id": inlineID, "rich_message": rich, "reply_markup": keyboard,
  })
  if err==nil || strings.Contains(strings.ToLower(err.Error()),"message is not modified") { return nil }
  log.Printf("Rich inline card edit failed: %v; sending a private fallback",err)
 }
 if messageID,ok:=navigationFromContext(ctx,chatID);ok{
  _,err:=b.call(ctx,"editMessageText",map[string]any{
   "chat_id":chatID,"message_id":messageID,"rich_message":rich,"reply_markup":keyboard,
  })
  if err==nil || strings.Contains(strings.ToLower(err.Error()),"message is not modified"){
   b.rememberBotScreenID(chatID,messageID)
   return nil
  }
  log.Printf("Rich card edit failed for %d: %v; sending new card",messageID,err)
 }
 raw,err:=b.call(ctx,"sendRichMessage",map[string]any{
  "chat_id":chatID,"rich_message":rich,"reply_markup":keyboard,
 })
 if err==nil {b.rememberBotScreen(chatID,raw)}
 return err
}

func regularCardRows(id string, backPage int, saved bool) [][]Button {
 label:="⭐ Добавить в избранное"
 if saved {label="✅ Убрать из избранного"}
 if backPage<0 {backPage=0}
 return [][]Button{
  {callbackButton(label,"fav:toggle:"+id+":"+strconv.Itoa(backPage))},
  {leadButton("card", id)},
  {callbackButton("⭐ Перейти в избранное","favorites"),callbackButton("🔎 Поиск локации","search:catalog")},
  {callbackButton("⬅️ К списку","page:"+strconv.Itoa(backPage)),menuButton()},
 }
}
