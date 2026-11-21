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

func TestMiniProfilePersistsAndAdminControlsDiscount(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"leads.json")
 store,err:=openLeadStore(path);if err!=nil{t.Fatal(err)}
 client:=&Bot{leads:store,adminIDs:map[int64]bool{90:true}}
 // Client profile is tied to the verified Telegram user ID.
 saved,err:=client.MiniSaveProfile(42,MiniProfile{Name:"Анна Иванова",Phone:"+7 906 528-89-35",Kind:"existing",DiscountPercent:90})
 if err!=nil{t.Fatal(err)}
 if saved.DiscountPercent!=0{t.Fatal("client changed discount without admin")}
 if saved.Phone!="+79065288935"{t.Fatalf("not normalized: %s",saved.Phone)}
 if _,err:=client.MiniAdminSetDiscount(42,42,30);err==nil{t.Fatal("non-admin changed discount")}
 if _,err:=client.MiniAdminSetDiscount(90,42,-1);err==nil{t.Fatal("negative discount accepted")}
 if _,err:=client.MiniAdminSetDiscount(90,42,101);err==nil{t.Fatal("discount above 100 accepted")}
 granted,err:=client.MiniAdminSetDiscount(90,42,10);if err!=nil{t.Fatal(err)}
 if granted.DiscountPercent!=10{t.Fatal("discount grant did not persist")}
 again,err:=client.MiniSaveProfile(42,MiniProfile{Name:"Анна Петрова",Phone:"+79065288935",Kind:"existing",DiscountPercent:0})
 if err!=nil{t.Fatal(err)}
 if again.DiscountPercent!=10{t.Fatal("client erased admin discount")}
 reopened,err:=openLeadStore(path);if err!=nil{t.Fatal(err)}
 got,ok:=(&Bot{leads:reopened}).MiniGetProfile(42)
 if !ok||got.DiscountPercent!=10||got.Name!="Анна Петрова"{t.Fatalf("profile did not survive restart: %+v %v",got,ok)}
}

func TestMiniProfileCreationFromCaseAndExistingHistory(t *testing.T){
 store,err:=openLeadStore(filepath.Join(t.TempDir(),"leads.json"));if err!=nil{t.Fatal(err)}
 b:=&Bot{leads:store,adminIDs:map[int64]bool{90:true}}
 b.MiniProfileFromCase(55,MiniCase{UserID:55,Name:"Пётр Иванов",Phone:"+79991112233",Kind:"new",Company:"ООО Компания"})
 p,ok:=b.MiniGetProfile(55);if !ok||p.Name!="Пётр Иванов"{t.Fatal("case should create profile")}
 b.MiniProfileFromCase(55,MiniCase{UserID:55,Name:"Неверное имя",Phone:"+79991112233",Kind:"new",Company:"ООО Компания"})
 p,_=b.MiniGetProfile(55);if p.Name!="Неверное имя"{t.Fatal("new case did not refresh profile contact data")}
 list,err:=b.MiniAdminProfiles(90);if err!=nil||len(list)!=1{t.Fatalf("admin profile list: %v %+v",err,list)}
 if _,err:=b.MiniAdminProfiles(55);err==nil{t.Fatal("non-admin can list profiles")}
}

func TestManualDiscountTelegramNotificationAfterRestart(t *testing.T) {
 t.Setenv("MINIAPP_URL","https://example.test/mini")
 path:=filepath.Join(t.TempDir(),"leads.json")
 store,err:=openLeadStore(path);if err!=nil{t.Fatal(err)}
 bot:=&Bot{leads:store,adminIDs:map[int64]bool{90:true}}
 if _,err=bot.MiniSaveProfile(42,MiniProfile{Name:"Анна Иванова",Phone:"+79990000000",Kind:"existing"});err!=nil{t.Fatal(err)}
 if _,err=bot.MiniAdminSetDiscount(90,42,15);err!=nil{t.Fatal(err)}
 if n:=len(store.manualDiscountNotices());n!=1{t.Fatalf("expected queued notice, got %d",n)}
 // A duplicate admin update must not generate a second notice.
 if _,err=bot.MiniAdminSetDiscount(90,42,15);err!=nil{t.Fatal(err)}
 store,err=openLeadStore(path);if err!=nil{t.Fatal(err)}
 bot.leads=store
 sent:=0
 fail:=true
 bot.http=&http.Client{Transport:referralTransport(func(req *http.Request)(*http.Response,error){
  if fail{return nil,errors.New("transient Telegram error")}
  var payload struct{ChatID int64 `json:"chat_id"`;Text string `json:"text"`;Markup string `json:"-"`}
  var data map[string]any
  if err:=json.NewDecoder(req.Body).Decode(&data);err!=nil{t.Fatal(err)}
  payload.ChatID=int64(data["chat_id"].(float64))
  payload.Text=data["text"].(string)
  if payload.ChatID!=42||!strings.Contains(payload.Text,"15%")||!strings.Contains(payload.Text,"назначена скидка") {t.Fatalf("unexpected notice: %+v",payload)}
  rows:=data["reply_markup"].(map[string]any)["inline_keyboard"].([]any)
  button:=rows[0].([]any)[0].(map[string]any)
  if !strings.Contains(button["text"].(string),"Использовать скидку"){t.Fatal("missing CTA",button)}
  if !strings.Contains(button["web_app"].(map[string]any)["url"].(string),"view=profile"){t.Fatal("no profile link",button)}
  sent++
  return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`))},nil
 })}
 bot.sendManualDiscountNotices(context.Background())
 if len(store.manualDiscountNotices())!=1{t.Fatal("failed notification was lost")}
 fail=false
 bot.sendManualDiscountNotices(context.Background())
 bot.sendManualDiscountNotices(context.Background())
 if sent!=1||len(store.manualDiscountNotices())!=0{t.Fatalf("duplicate notification or pending item: sent %d, queue %+v",sent,store.manualDiscountNotices())}
}
