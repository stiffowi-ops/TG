package telegram

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "html"
 "log"
 "os"
 "path/filepath"
 "sort"
 "strconv"
 "strings"
 "sync"
 "time"
)

// Telegram does not expose chat-open/chat-close events. A visit is an
// observable interaction after 30 minutes of inactivity, or a fresh /start
// at least five minutes after the previous /start.
const visitorIdleGap = 30 * time.Minute
const visitorStartGap = 5 * time.Minute
const visitorAlertDelay = time.Hour
const visitorRouteLimit = 35

var visitTZ = time.FixedZone("MSK", 3*60*60)

type visitorStep struct {
 At time.Time `json:"at"`
 Name string `json:"name"`
}
type visitorProfile struct {
 UserID int64 `json:"user_id"`
 FirstName string `json:"first_name,omitempty"`
 LastName string `json:"last_name,omitempty"`
 Username string `json:"username,omitempty"`
 FirstAt time.Time `json:"first_at"`
 LastAt time.Time `json:"last_at"`
 LastStart time.Time `json:"last_start,omitempty"`
 FirstRoute []visitorStep `json:"first_route,omitempty"`
 FirstRouteClosed bool `json:"first_route_closed,omitempty"`
 Daily map[string]int `json:"daily,omitempty"`
 HasLead bool `json:"has_lead,omitempty"`
 Alerted bool `json:"alerted,omitempty"`
 Hidden bool `json:"hidden,omitempty"`
 AlertRecipients map[string]bool `json:"alert_recipients,omitempty"`
}

type visitorStore struct {
 mu sync.Mutex
 path string
 users map[string]visitorProfile
}

func openVisitorStore(path string) (*visitorStore,error) {
 v:=&visitorStore{path:path,users:map[string]visitorProfile{}}
 raw,err:=os.ReadFile(path)
 if errors.Is(err,os.ErrNotExist){return v,nil}
 if err!=nil{return nil,err}
 if err:=json.Unmarshal(raw,&v.users);err!=nil{return nil,fmt.Errorf("decode visitors: %w",err)}
 if v.users==nil{v.users=map[string]visitorProfile{}}
 for id,profile:=range v.users {
  if profile.UserID<=0 || id!=strconv.FormatInt(profile.UserID,10){return nil,fmt.Errorf("invalid visitor ID %s",id)}
 }
 return v,nil
}

func cloneVisitor(p visitorProfile) visitorProfile {
 p.FirstRoute=append([]visitorStep(nil),p.FirstRoute...)
 daily:=map[string]int{}
 for d,n:=range p.Daily{daily[d]=n}
 p.Daily=daily
 sent:=map[string]bool{}
 for k,val:=range p.AlertRecipients{sent[k]=val}
 p.AlertRecipients=sent
 return p
}

func (v *visitorStore) saveLocked() error {
 if err:=os.MkdirAll(filepath.Dir(v.path),0700);err!=nil{return err}
 data,err:=json.MarshalIndent(v.users,"","  ");if err!=nil{return err}
 f,err:=os.CreateTemp(filepath.Dir(v.path),".visitors-*.tmp")
 if err!=nil{return err}
 temp:=f.Name()
 defer os.Remove(temp)
 if err=f.Chmod(0600);err!=nil{f.Close();return err}
 if _,err=f.Write(data);err!=nil{f.Close();return err}
 if err=f.Sync();err!=nil{f.Close();return err}
 if err=f.Close();err!=nil{return err}
 return os.Rename(temp,v.path)
}

func (v *visitorStore) record(u user,section string,isStart bool,at time.Time) error {
 if v==nil || u.ID<=0 || section==""{return nil}
 at=at.UTC()
 key:=strconv.FormatInt(u.ID,10)
 v.mu.Lock();defer v.mu.Unlock()
 p:=cloneVisitor(v.users[key])
 first:=p.FirstAt.IsZero()
 if first {
  p.UserID=u.ID
  p.FirstAt=at
  p.Daily=map[string]int{}
 }
 if u.FirstName!="" {p.FirstName=u.FirstName}
 if u.LastName!="" {p.LastName=u.LastName}
 if u.Username!="" {p.Username=u.Username}
 inactive:=!first && at.Sub(p.LastAt)>=visitorIdleGap
 newStart:=isStart && (p.LastStart.IsZero() || at.Sub(p.LastStart)>=visitorStartGap)
 newVisit:=first || inactive || newStart
 if newVisit {
  date:=at.In(visitTZ).Format("2006-01-02")
  p.Daily[date]++
  if !first {p.FirstRouteClosed=true}
 }
 if isStart {p.LastStart=at}
 if first || at.After(p.LastAt){p.LastAt=at}
 if !p.FirstRouteClosed && len(p.FirstRoute)<visitorRouteLimit {
  if n:=len(p.FirstRoute);n==0 || p.FirstRoute[n-1].Name!=section {
   p.FirstRoute=append(p.FirstRoute,visitorStep{At:at,Name:section})
  }
 }
 // Keep visit counts for 35 days and the route only for 30 days.
 for day:=range p.Daily{
  parsed,e:=time.ParseInLocation("2006-01-02",day,visitTZ)
  if e!=nil || at.Sub(parsed)>35*24*time.Hour {delete(p.Daily,day)}
 }
 if at.Sub(p.FirstAt)>30*24*time.Hour {p.FirstRoute=nil;p.FirstRouteClosed=true}
 v.users[key]=p
 return v.saveLocked()
}

// Periodic cleanup also covers users who never return. Keep only summary
// counters and one-time notification markers; clear the detailed first route
// after thirty days.
func (v *visitorStore) prune(now time.Time) error {
 if v==nil{return nil}
 v.mu.Lock();defer v.mu.Unlock()
 changed:=false
 for key,p:=range v.users {
  if !p.FirstAt.IsZero() && now.Sub(p.FirstAt)>30*24*time.Hour && len(p.FirstRoute)>0 {
   p.FirstRoute=nil;p.FirstRouteClosed=true;changed=true
  }
  for day:=range p.Daily {
   at,err:=time.ParseInLocation("2006-01-02",day,visitTZ)
   if err!=nil || now.Sub(at)>35*24*time.Hour {delete(p.Daily,day);changed=true}
  }
  v.users[key]=p
 }
 if !changed{return nil}
 return v.saveLocked()
}
func (v *visitorStore) markLead(userID int64) error {
 if v==nil||userID<=0{return nil}
 v.mu.Lock();defer v.mu.Unlock()
 key:=strconv.FormatInt(userID,10)
 p,ok:=v.users[key];if !ok{return nil}
 if p.HasLead {return nil}
 p.HasLead=true;v.users[key]=p
 return v.saveLocked()
}

func (v *visitorStore) noteAlertRecipient(userID,adminID int64,admins []int64) error {
 if v==nil{return nil}
 v.mu.Lock();defer v.mu.Unlock()
 key:=strconv.FormatInt(userID,10)
 p,ok:=v.users[key];if !ok{return nil}
 if p.AlertRecipients==nil{p.AlertRecipients=map[string]bool{}}
 p.AlertRecipients[strconv.FormatInt(adminID,10)]=true
 complete:=len(admins)>0
 for _,id:=range admins{if !p.AlertRecipients[strconv.FormatInt(id,10)] {complete=false}}
 p.Alerted=complete
 v.users[key]=p
 return v.saveLocked()
}

func (v *visitorStore) snapshot() []visitorProfile {
 if v==nil{return nil}
 v.mu.Lock();defer v.mu.Unlock()
 items:=make([]visitorProfile,0,len(v.users))
 for _,p:=range v.users{items=append(items,cloneVisitor(p))}
 sort.Slice(items,func(i,j int)bool{return items[i].LastAt.After(items[j].LastAt)})
 return items
}
func (v *visitorStore) find(id int64) (visitorProfile,bool) {
 if v==nil{return visitorProfile{},false}
 v.mu.Lock();defer v.mu.Unlock()
 p,ok:=v.users[strconv.FormatInt(id,10)]
 return cloneVisitor(p),ok
}
func (v *visitorStore) due(now time.Time) []visitorProfile {
 out:=[]visitorProfile{}
 for _,p:=range v.snapshot(){
  if !p.Hidden && !p.Alerted && !p.HasLead && !p.FirstAt.IsZero() && !now.Before(p.FirstAt.Add(visitorAlertDelay)){
   out=append(out,p)
  }
 }
 return out
}

func (b *Bot) observeVisitor(u user,action string,isStart bool) {
 if b.visitors==nil || u.ID<=0 || b.isAdmin(u.ID){return}
 if b.isPermanentlyDeletedVisitor(u.ID){return}
 if err:=b.visitors.record(u,action,isStart,time.Now());err!=nil{log.Printf("visitor tracking: %v",err)}
}
func (b *Bot) visitorCallbackAction(data string) string {
 switch {
 case data=="menu":return "Главное меню"
 case data=="list" || strings.HasPrefix(data,"page:"):return "Список локаций"
 case data=="near":return "Рядом со мной"
 case data=="favorites" || strings.HasPrefix(data,"fav:"):return "Избранное"
 case data=="search" || strings.HasPrefix(data,"search:"):return "Поиск локации"
 case strings.HasPrefix(data,"compare:"):return "Сравнение локаций"
 case strings.HasPrefix(data,"lead:"):return "Форма заявки"
 case strings.HasPrefix(data,"cab:"):return "Мои заявки"
 case strings.HasPrefix(data,"loc:"):
  id:=strings.Split(strings.TrimPrefix(data,"loc:"),":")[0]
  if b.store!=nil{if l,ok:=b.store.Find(id);ok{return "Карточка: "+limitRunes(publicText(l.Title),65)}}
  return "Карточка локации"
 default:return ""
 }
}

func visitorProfileURL(p visitorProfile) string {
 if p.Username!="" {
  name:=strings.TrimPrefix(strings.TrimSpace(p.Username),"@")
  valid:=len(name)>=5
  for _,c:=range name{if !(c>='a'&&c<='z'||c>='A'&&c<='Z'||c>='0'&&c<='9'||c=='_'){valid=false}}
  if valid{return "https://t.me/"+name}
 }
 return fmt.Sprintf("tg://user?id=%d",p.UserID)
}
func visitorToday(p visitorProfile,now time.Time) int {return p.Daily[now.In(visitTZ).Format("2006-01-02")]}
func visitorHTML(p visitorProfile,now time.Time) string {
 label:=func(s string)string{if strings.TrimSpace(s)==""{return "Не указано"};return html.EscapeString(s)}
 status:="Заявки нет"
 if p.HasLead{status="Заявка получена ✅"}
 var out strings.Builder
 out.WriteString("👤 <b>Посетитель</b>\n\n")
 out.WriteString("Имя: "+label(p.FirstName)+"\n")
 out.WriteString("Фамилия: "+label(p.LastName)+"\n")
 if p.Username==""{out.WriteString("Telegram: ник не указан\n")}else{out.WriteString("Telegram: @"+label(strings.TrimPrefix(p.Username,"@"))+"\n")}
 out.WriteString(fmt.Sprintf("Telegram ID: <code>%d</code>\n",p.UserID))
 out.WriteString("Телефон: не предоставлен боту\n")
 out.WriteString("Первый визит: "+p.FirstAt.In(visitTZ).Format("02.01.2006 15:04")+" МСК\n")
 out.WriteString("Последнее действие: "+p.LastAt.In(visitTZ).Format("02.01.2006 15:04")+" МСК\n")
 out.WriteString(fmt.Sprintf("Посещений сегодня: <b>%d</b>\n",visitorToday(p,now)))
 out.WriteString("Статус: <b>"+status+"</b>\n\n")
 out.WriteString("<b>Первый маршрут:</b>")
 if len(p.FirstRoute)==0 {out.WriteString("\nИстория старше 30 дней или отсутствует.")}
 for _,step:=range p.FirstRoute{out.WriteString("\n• "+step.At.In(visitTZ).Format("15:04")+" — "+label(step.Name))}
 return out.String()
}
func visitorKeyboard(p visitorProfile) map[string]any {
 return markup([]Button{urlButton("👤 Открыть профиль",visitorProfileURL(p))},
 []Button{callbackButton("📊 Карточка посетителя",fmt.Sprintf("adm:visitor:%d",p.UserID))})
}
func (b *Bot) visitorHasApplication(id int64) bool {
 if b.leads==nil{return false}
 for _,lead:=range b.leads.allSubmitted(){if lead.UserID==id{return true}}
 return false
}

func (b *Bot) sendVisitorAlerts(ctx context.Context,now time.Time) {
 if b.visitors==nil || len(b.adminIDs)==0{return}
 admins:=make([]int64,0,len(b.adminIDs))
 for id:=range b.adminIDs{admins=append(admins,id)}
 sort.Slice(admins,func(i,j int)bool{return admins[i]<admins[j]})
 for _,candidate:=range b.visitors.due(now) {
  if b.isAdmin(candidate.UserID)||b.isPermanentlyDeletedVisitor(candidate.UserID){continue}
  if b.visitorHasApplication(candidate.UserID) {
   if err:=b.visitors.markLead(candidate.UserID);err!=nil{log.Printf("visitor lead state: %v",err)}
   continue
  }
  // Reload from storage before each attempt, so state changed by a form
  // submission is observed even while the monitor is running.
  fresh,ok:=b.visitors.find(candidate.UserID)
  if !ok || fresh.Hidden || fresh.HasLead || fresh.Alerted{continue}
  for _,adminID:=range admins {
   fresh,ok=b.visitors.find(candidate.UserID)
   if !ok || fresh.Hidden || fresh.HasLead || fresh.Alerted{break}
   if fresh.AlertRecipients[strconv.FormatInt(adminID,10)]{continue}
   message:=b.visitorAlertHTML(fresh,now,b.visitorFavorites(fresh.UserID))
   _,err:=b.call(ctx,"sendMessage",map[string]any{
    "chat_id":adminID,"text":message,"parse_mode":"HTML",
    "link_preview_options":map[string]any{"is_disabled":true},
    "reply_markup":b.visitorAlertKeyboard(fresh),
   })
   if err!=nil{log.Printf("Visitor %d alert admin %d: %v",fresh.UserID,adminID,err);continue}
   if err:=b.visitors.noteAlertRecipient(fresh.UserID,adminID,admins);err!=nil {
    log.Printf("Save visitor alert delivery: %v",err)
   }
  }
 }
}
func (b *Bot) visitorMonitor(ctx context.Context) {
 if b.visitors!=nil {if err:=b.visitors.prune(time.Now());err!=nil{log.Printf("visitor cleanup: %v",err)}}
 b.sendVisitorAlerts(ctx,time.Now())
 ticker:=time.NewTicker(time.Minute)
 defer ticker.Stop()
 for {select{
  case <-ctx.Done():return
  case now:=<-ticker.C:
   if now.Minute()==0 && b.visitors!=nil {
    if err:=b.visitors.prune(now);err!=nil{log.Printf("visitor cleanup: %v",err)}
   }
   b.sendVisitorAlerts(ctx,now)
 }}
}

func (b *Bot) showVisitors(ctx context.Context,chatID int64) error {
 return b.showVisitorsPage(ctx,chatID,"pending",0)
}
func (b *Bot) showVisitorsPage(ctx context.Context,chatID int64,mode string,page int) error {
 if b.visitors==nil{return b.Send(ctx,chatID,"История посещений пока пуста.",markup([]Button{callbackButton("⬅️ Админ-панель","adm:menu")}))}
 users:=b.visitors.snapshot()
 now:=time.Now()
 unique7,noLead,withLead,visitsToday:=0,0,0,0
 cutoff:=now.AddDate(0,0,-7)
 filtered:=make([]visitorProfile,0,len(users))
 for _,p:=range users {
  if p.Hidden{continue}
  hasLead:=p.HasLead || b.visitorHasApplication(p.UserID)
  if hasLead {p.HasLead=true}
  if !p.LastAt.Before(cutoff){unique7++;if hasLead{withLead++}else{noLead++}}
  visitsToday+=visitorToday(p,now)
  if mode=="all" || !hasLead{filtered=append(filtered,p)}
 }
 if mode!="all"{mode="pending"}
 const pageSize=10
 pages:=max(1,(len(filtered)+pageSize-1)/pageSize)
 if page<0{page=0};if page>=pages{page=pages-1}
 rows:=[][]Button{
  {callbackButton("⏳ Без заявки","adm:visitors:pending:0"),callbackButton("👥 Все посетители","adm:visitors:all:0")},
 }
 start:=page*pageSize
 end:=min(start+pageSize,len(filtered))
 for _,p:=range filtered[start:end] {
  name:=strings.TrimSpace(p.FirstName+" "+p.LastName)
  if name==""{name=fmt.Sprintf("ID %d",p.UserID)}
  if p.HasLead{name+=" ✅"}
  rows=append(rows,[]Button{callbackButton("👤 "+limitRunes(name,29),fmt.Sprintf("adm:visitor:%d",p.UserID))})
 }
 nav:=[]Button{}
 if page>0{nav=append(nav,callbackButton("◀️ Назад",fmt.Sprintf("adm:visitors:%s:%d",mode,page-1)))}
 if page+1<pages{nav=append(nav,callbackButton("Далее ▶️",fmt.Sprintf("adm:visitors:%s:%d",mode,page+1)))}
 if len(nav)>0{rows=append(rows,nav)}
 rows=append(rows,[]Button{callbackButton("🗑 Выбрать посетителей","adm:clean:start:v:a"),callbackButton("♻️ Корзина","adm:clean:start:v:t")})
 rows=append(rows,[]Button{callbackButton("⬅️ Админ-панель","adm:menu")})
 title:="Без заявки";if mode=="all"{title="Все посетители"}
 return b.Send(ctx,chatID,fmt.Sprintf("👥 <b>Посетители</b>\n\nЗа последние 7 дней:\nУникальные: %d\nБез заявки: %d\nС заявкой: %d\nВизитов сегодня: %d\n\nРаздел: <b>%s</b> · %d записей · стр. %d/%d\n\nПовторные визиты обновляют счётчики, но не вызывают новый аларм.\n\nВизитом считается действие после 30 минут бездействия или повторный /start с интервалом от 5 минут. Пассивное открытие чата Telegram не показывает.",unique7,noLead,withLead,visitsToday,title,len(filtered),page+1,pages),markup(rows...))
}
func (b *Bot) showVisitor(ctx context.Context,chatID,userID int64) error {
 p,ok:=b.visitors.find(userID)
 if !ok || p.Hidden{return b.Send(ctx,chatID,"Посетитель не найден.",markup([]Button{callbackButton("⬅️ Посетители","adm:visitors")}))}
 if !p.HasLead && b.visitorHasApplication(userID){
  _=b.visitors.markLead(userID)
  p.HasLead=true
 }
 return b.Send(ctx,chatID,visitorHTML(p,time.Now()),markup(
  []Button{urlButton("👤 Открыть профиль",visitorProfileURL(p))},
  []Button{callbackButton("⭐ Избранные локации",fmt.Sprintf("adm:visitorfavs:%d:0",userID))},
  []Button{callbackButton("🗑 Скрыть запись",fmt.Sprintf("adm:clean:one:v:%d",userID))},
  []Button{callbackButton("⬅️ Посетители","adm:visitors")},
 ))
}

// setHidden atomically switches the display state. Hidden profiles still
// receive activity updates but cannot trigger additional no-lead alarms.
func (v *visitorStore) setHidden(ids []int64,hidden bool) error {
 if v==nil{return errors.New("visitor storage unavailable")}
 v.mu.Lock();defer v.mu.Unlock()
 copied:=make(map[string]visitorProfile,len(v.users))
 for k,p:=range v.users{copied[k]=cloneVisitor(p)}
 seen:=map[int64]bool{}
 for _,id:=range ids{
  if id<=0||seen[id]{return errors.New("invalid or duplicate visitor ID")}
  seen[id]=true
  key:=strconv.FormatInt(id,10)
  p,ok:=copied[key]
  if !ok{return fmt.Errorf("посетитель %d не найден",id)}
  if p.Hidden==hidden{return fmt.Errorf("посетитель %d уже имеет этот статус",id)}
  p.Hidden=hidden
  copied[key]=p
 }
 prior:=v.users
 v.users=copied
 if err:=v.saveLocked();err!=nil{v.users=prior;return err}
 return nil
}
func (v *visitorStore) eraseHidden(ids []int64) error {
 if v==nil{return errors.New("visitor storage unavailable")}
 v.mu.Lock();defer v.mu.Unlock()
 copied:=make(map[string]visitorProfile,len(v.users))
 for k,p:=range v.users{copied[k]=cloneVisitor(p)}
 seen:=map[int64]bool{}
 for _,id:=range ids{
  if id<=0||seen[id]{return errors.New("invalid or duplicate visitor ID")}
  seen[id]=true
  key:=strconv.FormatInt(id,10)
  p,ok:=copied[key]
  if !ok||!p.Hidden{return fmt.Errorf("посетитель %d не находится в корзине",id)}
  delete(copied,key)
 }
 prior:=v.users
 v.users=copied
 if err:=v.saveLocked();err!=nil{v.users=prior;return err}
 return nil
}

