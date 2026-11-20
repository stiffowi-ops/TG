package telegram

import (
 "errors"
 "sort"
 "strconv"
 "strings"
 "time"
 "unicode/utf8"
)

type miniProfileRecord struct {
 UserID int64 `json:"userId"`
 Name string `json:"name"`
 Phone string `json:"phone"`
 Company string `json:"company,omitempty"`
 Kind string `json:"kind"`
 DiscountPercent int `json:"discountPercent"`
 ManualDiscountPercent int `json:"manualDiscountPercent,omitempty"`
 ReferralDiscountPercent int `json:"referralDiscountPercent,omitempty"`
 UpdatedAt time.Time `json:"updatedAt"`
}

// Profile belongs to the authenticated Telegram user. Discount is admin-controlled.
type MiniProfile = miniProfileRecord

func validMiniProfile(p MiniProfile) error {
 p.Name=strings.TrimSpace(p.Name)
 if utf8.RuneCountInString(p.Name)<2||utf8.RuneCountInString(p.Name)>120 {return errors.New("укажите имя или ФИО (2–120 символов)")}
 if _,ok:=normalizeLeadPhone(p.Phone);!ok {return errors.New("проверьте номер: +7 и десять цифр")}
 if p.Kind!="existing"&&p.Kind!="new"{return errors.New("выберите действующего или потенциального партнёра")}
 if utf8.RuneCountInString(p.Company)>200{return errors.New("название организации слишком длинное")}
 return nil
}
func caseProfile(l leadApplication) MiniProfile {
 return MiniProfile{UserID:l.UserID,Name:l.Name,Phone:l.Phone,Company:l.Company,Kind:l.Kind}
}
func (s *leadStore) profileLocked(id int64) (MiniProfile,bool) {
 key:=strconv.FormatInt(id,10)
 if p,ok:=s.state.Profiles[key];ok{return p,true}
 if p,ok:=s.state.Submitted[key];ok{return caseProfile(p),true}
 for i:=len(s.state.History)-1;i>=0;i-- {
  if s.state.History[i].UserID==id{return caseProfile(s.state.History[i]),true}
 }
 return MiniProfile{},false
}
func (b *Bot) MiniGetProfile(id int64) (MiniProfile,bool) {
 if b==nil||b.leads==nil||id<=0{return MiniProfile{},false}
 b.leads.mu.Lock();defer b.leads.mu.Unlock()
 p,ok:=b.leads.profileLocked(id)
 if !ok {
  // Inviting clients may earn discounts before completing their contact profile.
  p=MiniProfile{UserID:id}
  p=b.leads.effectiveProfileLocked(p)
  if p.ReferralDiscountPercent==0{return MiniProfile{},false}
 }
 return b.leads.effectiveProfileLocked(p),true
}
func (b *Bot) MiniSaveProfile(id int64,p MiniProfile)(MiniProfile,error) {
 if b==nil||b.leads==nil||id<=0{return MiniProfile{},errors.New("доступ запрещён")}
 p.UserID=id
 p.Name=strings.TrimSpace(p.Name);p.Phone=strings.TrimSpace(p.Phone);p.Company=strings.TrimSpace(p.Company)
 if err:=validMiniProfile(p);err!=nil{return MiniProfile{},err}
 p.Phone,_=normalizeLeadPhone(p.Phone)
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 existing,_:=s.profileLocked(id)
 p.DiscountPercent=existing.DiscountPercent // never trust client-submitted discount
 p.UpdatedAt=time.Now().UTC()
 next:=s.cloneLocked()
 next.Profiles[strconv.FormatInt(id,10)]=p
 if err:=s.saveLocked(next);err!=nil{return MiniProfile{},err}
 return s.effectiveProfileLocked(p),nil
}
func (b *Bot) MiniProfileFromCase(userID int64, l MiniCase) {
 if b==nil||b.leads==nil||userID<=0{return}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 existing,_:=s.profileLocked(userID)
 // The most recently submitted mini-app case refreshes contact details,
 // but never overwrites the admin-controlled discount.
 p:=MiniProfile{UserID:userID,Name:l.Name,Phone:l.Phone,Company:l.Company,Kind:l.Kind,DiscountPercent:existing.DiscountPercent,UpdatedAt:time.Now().UTC()}
 if validMiniProfile(p)!=nil{return}
 next:=s.cloneLocked();next.Profiles[strconv.FormatInt(userID,10)]=p
 _=s.saveLocked(next)
}
func (b *Bot) MiniAdminProfiles(adminID int64)([]MiniProfile,error){
 if !b.IsMiniAdmin(adminID){return nil,errors.New("доступ запрещён")}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 byID:=map[int64]MiniProfile{}
 // Legacy requests are included so clients do not disappear from the bank.
 for _,l:=range s.state.History {if l.UserID>0&&!l.Hidden{byID[l.UserID]=caseProfile(l)}}
 for _,l:=range s.state.Submitted {if l.UserID>0&&!l.Hidden{byID[l.UserID]=caseProfile(l)}}
 for _,p:=range s.state.Profiles {if p.UserID>0{byID[p.UserID]=p}}
 for id,p:=range byID {byID[id]=s.effectiveProfileLocked(p)}
 out:=make([]MiniProfile,0,len(byID))
 for _,p:=range byID {out=append(out,p)}
 sort.Slice(out,func(i,j int)bool{return out[i].UserID<out[j].UserID})
 return out,nil
}
func (b *Bot) MiniAdminSetDiscount(adminID,userID int64,discount int)(MiniProfile,error){
 if !b.IsMiniAdmin(adminID){return MiniProfile{},errors.New("доступ запрещён")}
 if userID<=0||discount<0||discount>100{return MiniProfile{},errors.New("скидка должна быть от 0 до 100%")}
 s:=b.leads;s.mu.Lock();defer s.mu.Unlock()
 p,ok:=s.profileLocked(userID)
 if !ok{return MiniProfile{},errors.New("профиль клиента не найден")}
 p.UserID=userID;p.DiscountPercent=discount;p.UpdatedAt=time.Now().UTC()
 next:=s.cloneLocked();next.Profiles[strconv.FormatInt(userID,10)]=p
 if err:=s.saveLocked(next);err!=nil{return MiniProfile{},err}
 return s.effectiveProfileLocked(p),nil
}
