package telegram

import (
 "context"
 "strings"
 "testing"
 "time"
)

func moscowTime(y int, month time.Month, day, hour, minute int) time.Time {
 return time.Date(y,month,day,hour,minute,0,0,visitTZ)
}

func TestCaseTakeSLA30WorkingMinutes(t *testing.T) {
 cases:=[]struct{name string;created,now time.Time;stage string}{
  {"before SLA",moscowTime(2026,time.November,23,10,0),moscowTime(2026,time.November,23,10,29),""},
  {"at SLA",moscowTime(2026,time.November,23,10,0),moscowTime(2026,time.November,23,10,30),"sla30"},
  {"2h reminder",moscowTime(2026,time.November,23,10,0),moscowTime(2026,time.November,23,12,0),"reminder"},
  {"4h escalation",moscowTime(2026,time.November,23,10,0),moscowTime(2026,time.November,23,14,0),"escalation"},
  {"after work no reminder",moscowTime(2026,time.November,23,18,45),moscowTime(2026,time.November,23,19,30),""},
  {"next day remaining 15 minutes",moscowTime(2026,time.November,23,18,45),moscowTime(2026,time.November,24,9,14),""},
  {"next day SLA",moscowTime(2026,time.November,23,18,45),moscowTime(2026,time.November,24,9,15),"sla30"},
  {"weekend paused",moscowTime(2026,time.November,20,18,50),moscowTime(2026,time.November,22,14,0),""},
  {"monday remaining 20",moscowTime(2026,time.November,20,18,50),moscowTime(2026,time.November,23,9,19),""},
  {"monday at threshold",moscowTime(2026,time.November,20,18,50),moscowTime(2026,time.November,23,9,20),"sla30"},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){got:=caseTakeReminderStage(tc.created,tc.now);if got!=tc.stage{t.Fatalf("expected %q, got %q",tc.stage,got)}})}
 // Existing 2h/4h supplement reminders do not change.
 if got:=adminReminderStage(moscowTime(2026,time.November,23,10,0),moscowTime(2026,time.November,23,10,30));got!="" {t.Fatalf("supplement stage changed: %q",got)}
}
func TestClientTakeContactLanguageAndSupplementDetails(t *testing.T) {
 working:=moscowTime(2026,time.November,23,11,0)
 outside:=moscowTime(2026,time.November,23,20,0)
 if text:=clientContactExpectation(leadApplication{TakenAt:&working});!strings.Contains(text,"в ближайшее время"){t.Fatal(text)}
 if text:=clientContactExpectation(leadApplication{TakenAt:&outside});!strings.Contains(text,"рабочее время"){t.Fatal(text)}
 summary:=clientSupplementDetails(leadUpdate{
  Comment:"Уточните <вопрос>",
  Locations:[]leadLocation{{Name:"Метро Сколково"},{Name:"Москва & область"}},
 })
 if !strings.Contains(summary,"Метро Сколково")||!strings.Contains(summary,"Уточните &lt;вопрос&gt;")||!strings.Contains(summary,"Москва &amp; область") {t.Fatal(summary)}
 if s:=pendingCaseReminderText(leadApplication{Number:1,Name:"Тест",Phone:"+79990000000"}, "sla30");!strings.Contains(s,"SLA 30 минут"){t.Fatal(s)}
}
func TestDiscountButtonOpensDiscountScreen(t *testing.T) {
 t.Setenv("MINIAPP_URL","https://example.test/app?lang=ru")
 b:=&Bot{}
 btn:=b.discountButton(context.Background(),"🎁 Использовать скидку")
 app,ok:=btn["web_app"].(map[string]string)
 if !ok||app["url"]!="https://example.test/app?lang=ru&view=discount"{t.Fatalf("wrong link: %+v",btn)}
}
