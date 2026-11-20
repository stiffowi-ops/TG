package telegram

import (
 "context"
 "crypto/rand"
 "encoding/base64"
 "encoding/json"
 "errors"
 "fmt"
 "html"
 "log"
 "net/url"
 "os"
 "regexp"
 "strconv"
 "strings"
 "time"
)

const referralAwardPercent = 10

type referralRecord struct {
 InviteeID int64 `json:"inviteeId"`
 InviterID int64 `json:"inviterId"`
 CaseNumber int64 `json:"caseNumber"`
 InviterName string `json:"inviterName"`
 InviteeName string `json:"inviteeName"`
 Code string `json:"code"`
 Status string `json:"status"` // pending, deferred, awarded
 Percent int `json:"percent,omitempty"`
 DecidedBy int64 `json:"decidedBy,omitempty"`
 CreatedAt time.Time `json:"createdAt"`
 DecidedAt *time.Time `json:"decidedAt,omitempty"`
 Notified bool `json:"notified,omitempty"`
}
type MiniReferralInfo struct {
 Code string `json:"code"`
 Link string `json:"link"`
 PendingCode string `json:"pendingCode,omitempty"`
 Pending int `json:"pending"`
 Awarded int `json:"awarded"`
}
type MiniReferralValidation struct {
 Code string `json:"code"`
 Valid bool `json:"valid"`
}
var referralCodePattern=regexp.MustCompile(`^ref_[A-Za-z0-9_-]{16}$`)
var botUsernamePattern=regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

func parseReferralCode(raw string) (string,error) {
 raw=strings.TrimSpace(raw)
 if len(raw)>512 {return "",errors.New("слишком длинная ссылка")}
 if strings.HasPrefix(raw,"https://") || strings.HasPrefix(raw,"http://") {
  parsed,err:=url.Parse(raw)
  if err!=nil || parsed.Scheme!="https" || !strings.EqualFold(parsed.Hostname(),"t.me") || parsed.Port()!="" || parsed.User!=nil {return "",errors.New("используйте реферальную ссылку Telegram")}
  raw=parsed.Query().Get("startapp")
 }
 if !referralCodePattern.MatchString(raw){return "",errors.New("неверный реферальный код")}
 return raw,nil
}
func (s *leadStore) referralOwnerLocked(code string) int64 {
 for uid,stored:=range s.state.ReferralCodes {
  if stored==code {id,_:=strconv.ParseInt(uid,10,64);return id}
 }
 return 0
}
func (s *leadStore) alreadyAppliedLocked(id int64) bool {
 key:=strconv.FormatInt(id,10)
 if _,ok:=s.state.Referrals[key];ok{return true}
 if _,ok:=s.state.Submitted[key];ok{return true}
 for _,l:=range s.state.History {if l.UserID==id{return true}}
 return false
}
func (s *leadStore) referralDisplayNameLocked(id int64) string {
 if p,ok:=s.profileLocked(id);ok&&strings.TrimSpace(p.Name)!="" {return p.Name}
 return fmt.Sprintf("Клиент %d",id)
}
func (s *leadStore) newReferralCode(id int64)(string,error) {
 s.mu.Lock();defer s.mu.Unlock()
 key:=strconv.FormatInt(id,10)
 if code:=s.state.ReferralCodes[key];code!=""{return code,nil}
 next:=s.cloneLocked()
 for attempts:=0;attempts<10;attempts++ {
  raw:=make([]byte,12)
  if _,err:=rand.Read(raw);err!=nil{return "",err}
  code:="ref_"+base64.RawURLEncoding.EncodeToString(raw)
  if s.referralOwnerLocked(code)!=0{continue}
  next.ReferralCodes[key]=code
  if err:=s.saveLocked(next);err!=nil{return "",err}
  return code,nil
 }
 return "",errors.New("не удалось создать уникальный код")
}
func (b *Bot) referralUsername(ctx context.Context)(string,error) {
 if b==nil{return "",errors.New("бот недоступен")}
 b.botIdentityMu.RLock();name:=b.botUsername;b.botIdentityMu.RUnlock()
 if name==""{name=strings.TrimPrefix(strings.TrimSpace(os.Getenv("REFERRAL_BOT_USERNAME")),"@")}
 if name==""{
  raw,err:=b.call(ctx,"getMe",map[string]any{})
  if err!=nil{return "",err}
  var identity struct{Username string `json:"username"`}
  if err=json.Unmarshal(raw,&identity);err!=nil{return "",err}
  name=identity.Username
  b.botIdentityMu.Lock();b.botUsername=name;b.botIdentityMu.Unlock()
 }
 if !botUsernamePattern.MatchString(name){return "",errors.New("имя Telegram-бота не настроено")}
 return name,nil
}
func (b *Bot) MiniReferralInfo(ctx context.Context,id int64)(MiniReferralInfo,error) {
 if b==nil||b.leads==nil||id<=0{return MiniReferralInfo{},errors.New("доступ запрещён")}
 code,err:=b.leads.newReferralCode(id)
 if err!=nil{return MiniReferralInfo{},err}
 name,err:=b.referralUsername(ctx)
 if err!=nil{return MiniReferralInfo{},err}
 info:=MiniReferralInfo{Code:code,Link:"https://t.me/"+name+"?startapp="+code}
 b.leads.mu.Lock();defer b.leads.mu.Unlock()
 info.PendingCode=b.leads.state.PendingReferrals[strconv.FormatInt(id,10)]
 for _,rec:=range b.leads.state.Referrals{
  if rec.InviterID!=id {continue}
  if rec.Status=="awarded"{info.Awarded++}else{info.Pending++}
 }
 return info,nil
}
func (b *Bot) MiniValidateReferral(id int64,raw string)(MiniReferralValidation,error) {
 code,err:=parseReferralCode(raw);if err!=nil{return MiniReferralValidation{},err}
 if b==nil||b.leads==nil||id<=0{return MiniReferralValidation{},errors.New("доступ запрещён")}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 owner:=s.referralOwnerLocked(code)
 if owner==0 {return MiniReferralValidation{},errors.New("реферальная ссылка не найдена")}
 if owner==id{return MiniReferralValidation{},errors.New("свою ссылку использовать нельзя")}
 if s.alreadyAppliedLocked(id){return MiniReferralValidation{},errors.New("приглашение для вашего аккаунта уже использовано")}
 key:=strconv.FormatInt(id,10)
 if previous:=s.state.PendingReferrals[key];previous!=""&&previous!=code{return MiniReferralValidation{},errors.New("для вас уже закреплено другое приглашение")}
 return MiniReferralValidation{Code:code,Valid:true},nil
}
// Accept only the start_param from a cryptographically verified Telegram initData.
func (b *Bot) MiniCaptureReferral(id int64,startParam string)(string,error) {
 if !strings.HasPrefix(startParam,"ref_"){return "",nil}
 v,err:=b.MiniValidateReferral(id,startParam);if err!=nil{return "",err}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 key:=strconv.FormatInt(id,10)
 if stored:=s.state.PendingReferrals[key];stored!=""{return stored,nil}
 next:=s.cloneLocked();next.PendingReferrals[key]=v.Code
 if err:=s.saveLocked(next);err!=nil{return "",err}
 return v.Code,nil
}
func (b *Bot) MiniPendingReferral(id int64) string {
 if b==nil||b.leads==nil||id<=0{return ""}
 b.leads.mu.Lock();defer b.leads.mu.Unlock()
 return b.leads.state.PendingReferrals[strconv.FormatInt(id,10)]
}
func (b *Bot) MiniClearPendingReferral(id int64)error {
 if b==nil||b.leads==nil||id<=0{return errors.New("доступ запрещён")}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 key:=strconv.FormatInt(id,10)
 if s.state.PendingReferrals[key]==""{return nil}
 next:=s.cloneLocked();delete(next.PendingReferrals,key)
 return s.saveLocked(next)
}
func (b *Bot) MiniPrepareReferral(id int64,raw string)error {
 if strings.TrimSpace(raw)==""{return nil}
 v,err:=b.MiniValidateReferral(id,raw);if err!=nil{return err}
 _,err=b.MiniCaptureReferral(id,v.Code)
 return err
}
func (s *leadStore) effectiveProfileLocked(p MiniProfile)MiniProfile {
 manual:=p.DiscountPercent
 referral:=0
 for _,rec:=range s.state.Referrals{
  if rec.InviterID==p.UserID&&rec.Status=="awarded"&&rec.Percent>referral {referral=rec.Percent}
 }
 p.ManualDiscountPercent=manual
 p.ReferralDiscountPercent=referral
 if referral>manual {p.DiscountPercent=referral}
 return p
}
func (s *leadStore) referralByCaseLocked(number int64)(referralRecord,bool){
 for _,rec:=range s.state.Referrals {if rec.CaseNumber==number{return rec,true}}
 return referralRecord{},false
}
func (b *Bot) MiniAdminReferralAction(adminID,number int64,action string)(referralRecord,bool,error){
 if !b.IsMiniAdmin(adminID){return referralRecord{},false,errors.New("доступ запрещён")}
 if action!="award"&&action!="defer"{return referralRecord{},false,errors.New("неверное действие")}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 rec,ok:=s.referralByCaseLocked(number)
 if !ok{return referralRecord{},false,errors.New("реферальное приглашение не найдено")}
 lead,ok:=s.leadByNumberLocked(number)
 if !ok||lead.Hidden{return referralRecord{},false,errors.New("заявка недоступна")}
 if !lead.TakenInWork{return referralRecord{},false,errors.New("сначала возьмите заявку в работу")}
 if rec.Status=="awarded" {
  if action=="award"{return rec,false,nil}
  return rec,false,errors.New("скидка уже начислена")
 }
 if rec.Status=="deferred"&&action=="defer"{return rec,false,nil}
 now:=time.Now().UTC()
 rec.DecidedAt=&now;rec.DecidedBy=adminID
 if action=="award"{rec.Status="awarded";rec.Percent=referralAwardPercent}else{rec.Status="deferred";rec.Percent=0}
 next:=s.cloneLocked()
 next.Referrals[strconv.FormatInt(rec.InviteeID,10)]=rec
 key:=strconv.FormatInt(rec.InviteeID,10)
 if lead,ok:=next.Submitted[key];ok&&lead.Number==number{
  lead.ReferralStatus=rec.Status;next.Submitted[key]=lead
 }else{
  for i:=range next.History{if next.History[i].Number==number{next.History[i].ReferralStatus=rec.Status;break}}
 }
 if err:=s.saveLocked(next);err!=nil{return referralRecord{},false,err}
 return rec,true,nil
}
func (s *leadStore) leadByNumberLocked(number int64)(leadApplication,bool) {
 for _,l:=range s.state.Submitted {if l.Number==number{return l,true}}
 for _,l:=range s.state.History {if l.Number==number{return l,true}}
 return leadApplication{},false
}
func (b *Bot) MiniReferralAwarded(rec referralRecord) {
 if b==nil{return}
 select{case b.referralWake<-struct{}{}:default:}
 b.queueLeadDelivery(rec.CaseNumber)
}
func (b *Bot) MiniDeferredReminder(ctx context.Context,adminID int64,rec referralRecord) {
 if b==nil{return}
 _,err:=b.call(ctx,"sendMessage",map[string]any{"chat_id":adminID,"text":fmt.Sprintf("🕓 <b>Реферальная скидка отложена</b>\n\nЗаявка №%04d, приглашённый: %s.\nПригласивший: %s.\n\nСкидка не начислена. Вернитесь к заявке, чтобы назначить 10%% вручную.",rec.CaseNumber,html.EscapeString(rec.InviteeName),html.EscapeString(rec.InviterName)),"parse_mode":"HTML","reply_markup":markup([]Button{adminCaseButton(rec.CaseNumber)})})
 if err!=nil{log.Printf("Referral deferred reminder %d: %v",rec.CaseNumber,err)}
}
func (s *leadStore) referralNotices()[]referralRecord {
 s.mu.Lock();defer s.mu.Unlock()
 result:=[]referralRecord{}
 for _,rec:=range s.state.Referrals{if rec.Status=="awarded"&&!rec.Notified{result=append(result,rec)}}
 return result
}
func (s *leadStore) markReferralNotified(inviteeID,caseNumber int64)error {
 s.mu.Lock();defer s.mu.Unlock()
 key:=strconv.FormatInt(inviteeID,10)
 rec,ok:=s.state.Referrals[key]
 if !ok||rec.CaseNumber!=caseNumber||rec.Status!="awarded"{return errors.New("referral not found")}
 if rec.Notified{return nil}
 next:=s.cloneLocked();rec.Notified=true;next.Referrals[key]=rec
 return s.saveLocked(next)
}
func (b *Bot) sendReferralNotices(ctx context.Context) {
 if b==nil||b.leads==nil{return}
 for _,rec:=range b.leads.referralNotices(){
  if ctx.Err()!=nil{return}
  text:=fmt.Sprintf("🎉 <b>Вам начислена скидка 10%%!</b>\n\nПо вашему приглашению поступила заявка нового клиента. Скидка доступна на следующую оплату.")
  button:=Button{}
  if appURL:=strings.TrimSpace(os.Getenv("MINIAPP_URL"));strings.HasPrefix(appURL,"https://"){
   parsed,err:=url.Parse(appURL)
   if err==nil&&parsed.Host!="" {q:=parsed.Query();q.Set("view","profile");parsed.RawQuery=q.Encode();button=Button{"text":"👤 Перейти в профиль","web_app":map[string]string{"url":parsed.String()}}}
  }
  if len(button)==0 {
   name,err:=b.referralUsername(ctx)
   if err==nil{button=urlButton("👤 Перейти в профиль","https://t.me/"+name+"?startapp=profile")}
  }
  var keyboard any
  if len(button)>0{keyboard=markup([]Button{button})}
  _,err:=b.call(ctx,"sendMessage",map[string]any{"chat_id":rec.InviterID,"text":text,"parse_mode":"HTML","reply_markup":keyboard})
  if err!=nil{log.Printf("Referral notice to %d (case %d): %v",rec.InviterID,rec.CaseNumber,err);continue}
  if err:=b.leads.markReferralNotified(rec.InviteeID,rec.CaseNumber);err!=nil{log.Printf("Save referral notice receipt: %v",err)}
 }
}
func (b *Bot) runReferralNotices(ctx context.Context) {
 ticker:=time.NewTicker(2*time.Minute);defer ticker.Stop()
 b.sendReferralNotices(ctx)
 for {
  select{
  case <-ctx.Done():return
  case <-ticker.C:b.sendReferralNotices(ctx)
  case <-b.referralWake:b.sendReferralNotices(ctx)
  }
 }
}

func (b *Bot) handleReferralAdminCallback(ctx context.Context, cb *callbackQuery)error {
 if b==nil||cb==nil||cb.From.ID<=0||!b.IsMiniAdmin(cb.From.ID)||cb.Message==nil||cb.Message.Chat.Type!="private"||cb.Message.Chat.ID!=cb.From.ID{return nil}
 parts:=strings.Split(cb.Data,":")
 if len(parts)!=3||parts[0]!="ref"{return nil}
 number,err:=strconv.ParseInt(parts[2],10,64);if err!=nil||number<=0{return nil}
 action:=parts[1]
 rec,changed,err:=b.MiniAdminReferralAction(cb.From.ID,number,action)
 if err!=nil{return b.Send(ctx,cb.From.ID,html.EscapeString(err.Error()),markup([]Button{adminCaseButton(number)}))}
 if changed {
  if action=="award"{b.MiniReferralAwarded(rec)}
  if action=="defer"{b.MiniDeferredReminder(ctx,cb.From.ID,rec)}
 }
 return b.showAdminCase(ctx,cb.From.ID,number,0)
}
