package telegram

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// MiniCase is the API-facing projection of the existing Karty application.
// The client only receives cases belonging to its authenticated Telegram ID;
// administrator lists are guarded separately by IsMiniAdmin.
type MiniCase struct {
	Number int64 `json:"number"`
	UserID int64 `json:"userId,omitempty"`
	Username string `json:"username,omitempty"`
	Name string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
	Company string `json:"company,omitempty"`
	Kind string `json:"kind,omitempty"`
	LocationIDs []string `json:"locationIds"`
	CreatedAt time.Time `json:"createdAt"`
	Status string `json:"status"`
	TakenBy int64 `json:"takenBy,omitempty"`
	TakenAt *time.Time `json:"takenAt,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	Updates []MiniUpdate `json:"updates"`
 ReferrerID int64 `json:"referrerId,omitempty"`
 ReferrerName string `json:"referrerName,omitempty"`
 ReferralStatus string `json:"referralStatus,omitempty"`
}

type MiniUpdate struct {
	ID int `json:"id"`
	LocationIDs []string `json:"locationIds"`
	Comment string `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
}

func miniCase(l leadApplication) MiniCase {
	status := "received"
	if l.isResolved() { status = "resolved" } else if l.TakenInWork { status = "working" }
	out := MiniCase{
		Number:l.Number, UserID:l.UserID, Username:l.Username,
		Name:l.Name, Phone:l.Phone, Company:l.Company, Kind:l.Kind,
		CreatedAt:l.CreatedAt, Status:status, TakenBy:l.TakenBy,
		TakenAt:l.TakenAt, ResolvedAt:l.ResolvedAt,
 ReferrerID:l.ReferrerID,ReferrerName:l.ReferrerName,ReferralStatus:l.ReferralStatus,
		LocationIDs:[]string{}, Updates:[]MiniUpdate{},
	}
	for _,loc:=range l.locations() { out.LocationIDs=append(out.LocationIDs,loc.ID) }
	for _,u:=range l.Updates {
		m:=MiniUpdate{ID:u.ID,Comment:u.Comment,CreatedAt:u.CreatedAt,AcceptedAt:u.AcceptedAt,LocationIDs:[]string{}}
		for _,loc:=range u.Locations { m.LocationIDs=append(m.LocationIDs,loc.ID) }
		out.Updates=append(out.Updates,m)
	}
	return out
}

func (b *Bot) IsMiniAdmin(id int64) bool {
	return b != nil && b.isAdmin(id)
}

func (b *Bot) MiniFavorites(userID int64) []string {
	if b==nil || b.favorites==nil || userID<=0 {return []string{}}
	ids := b.favorites.IDs(userID)
	if ids == nil {return []string{}}
	return ids
}

func (b *Bot) MiniToggleFavorite(userID int64, locationID string) ([]string,error) {
	if b==nil || b.favorites==nil || userID<=0 { return nil, errors.New("доступ запрещён") }
	if _,ok:=b.store.Find(locationID); !ok {return nil, errors.New("локация не найдена")}
	added,err:=b.favorites.Toggle(userID,locationID);if err!=nil{return nil,err}
	if added {b.recordLocationAnalytics(userID,locationID,"favorite_add")} else {b.recordLocationAnalytics(userID,locationID,"favorite_remove")}
	return b.MiniFavorites(userID),nil
}

func (b *Bot) MiniMyCases(userID int64) []MiniCase {
	if b==nil || b.leads==nil || userID<=0 {return []MiniCase{}}
	result:=[]MiniCase{}
	if current,ok:=b.leads.submitted(userID);ok {result=append(result,miniCase(current))}
	b.leads.mu.Lock()
	for _,l:=range b.leads.state.History {
		if l.UserID==userID {result=append(result,miniCase(l))}
	}
	b.leads.mu.Unlock()
	sort.Slice(result,func(i,j int)bool{return result[i].Number>result[j].Number})
	return result
}

func (b *Bot) MiniAdminCases(adminID int64) ([]MiniCase,error) {
	if !b.IsMiniAdmin(adminID) {return nil,errors.New("доступ запрещён")}
	out:=[]MiniCase{}
	for _,l:=range b.leads.visibleSubmitted(){out=append(out,miniCase(l))}
	sort.Slice(out,func(i,j int)bool{return out[i].Number>out[j].Number})
	return out,nil
}

type MiniNewCase struct {
	LocationID string `json:"locationId"`
	Kind string `json:"kind"`
	Company string `json:"company"`
	Name string `json:"name"`
	Phone string `json:"phone"`
 ReferralCode string `json:"referralCode,omitempty"`
}

func (b *Bot) MiniCreateCase(userID int64, username string, input MiniNewCase) (MiniCase,error) {
	if b==nil || b.leads==nil || userID<=0 {return MiniCase{},errors.New("доступ запрещён")}
	if len(b.adminIDs)==0 {return MiniCase{},errors.New("приём заявок временно недоступен")}
	loc,ok:=b.store.Find(input.LocationID);if !ok {return MiniCase{},errors.New("локация не найдена")}
	input.Kind=strings.TrimSpace(input.Kind)
	input.Name=strings.TrimSpace(input.Name)
	input.Company=strings.TrimSpace(input.Company)
	if input.Kind!="new" && input.Kind!="existing" {return MiniCase{},errors.New("укажите тип сотрудничества")}
	if utf8.RuneCountInString(input.Name)<2 || utf8.RuneCountInString(input.Name)>120 {return MiniCase{},errors.New("проверьте имя")}
	if input.Kind=="new" && (utf8.RuneCountInString(input.Company)<2 || utf8.RuneCountInString(input.Company)>200) {return MiniCase{},errors.New("укажите юридическое лицо")}
	phone,ok:=normalizeLeadPhone(input.Phone);if !ok {return MiniCase{},errors.New("номер должен содержать +7 и десять цифр")}
	if current,ok:=b.leads.submitted(userID);ok&&!current.isResolved() {return miniCase(current),errors.New("у Вас уже есть активное обращение")}
 if err:=b.MiniPrepareReferral(userID,input.ReferralCode);err!=nil{return MiniCase{},err}
	d:=leadDraft{Source:"menu",Stage:"review",Kind:input.Kind,Company:input.Company,Name:input.Name,Phone:phone,LocationID:loc.ID}
	if err:=b.leads.putDraft(userID,d);err!=nil{return MiniCase{},err}
	l,already,err:=b.leads.submit(userID,username,publicText(loc.Title))
	if err!=nil{return MiniCase{},err}
	if already{return miniCase(l),errors.New("заявка уже существует")}
	if b.visitors!=nil { _=b.visitors.markLead(userID) }
	b.queueLeadDelivery(l.Number)
	return miniCase(l),nil
}

type MiniSupplement struct {
	LocationIDs []string `json:"locationIds"`
	Comment string `json:"comment"`
}
func (b *Bot) MiniAddSupplement(userID,number int64,input MiniSupplement)(MiniCase,error){
	if b==nil || b.leads==nil || userID<=0{return MiniCase{},errors.New("доступ запрещён")}
	if len(input.LocationIDs)>10 || utf8.RuneCountInString(input.Comment)>1000 {return MiniCase{},errors.New("превышен лимит дополнения")}
	cur,ok:=b.leads.submitted(userID);if !ok||cur.Number!=number{return MiniCase{},errors.New("заявка недоступна")}
	d,err:=newSupplement(number);if err!=nil{return MiniCase{},err}
	d.Stage="review"
	d.Comment=strings.TrimSpace(input.Comment)
	d.Locations=[]leadLocation{}
	seen:=map[string]bool{}
	for _,id:=range input.LocationIDs{
		if seen[id] {continue};seen[id]=true
		loc,ok:=b.store.Find(id);if !ok {return MiniCase{},fmt.Errorf("локация %s не найдена",id)}
		d.Locations=append(d.Locations,leadLocation{ID:id,Name:publicText(loc.Title)})
	}
	if len(d.Locations)==0&&d.Comment==""{return MiniCase{},errors.New("добавьте локацию или комментарий")}
	if err:=b.leads.putSupplement(userID,d);err!=nil{return MiniCase{},err}
	l,changed,err:=b.leads.submitSupplement(userID,number,d.Token);if err!=nil{return MiniCase{},err}
	if changed{b.queueLeadDelivery(number)}
	return miniCase(l),nil
}

func (b *Bot) MiniAdminAction(adminID,number int64,action string,updateID int)(MiniCase,error){
	if !b.IsMiniAdmin(adminID){return MiniCase{},errors.New("доступ запрещён")}
	var l leadApplication
	var changed bool
	var err error
	switch action {
	case "take":l,changed,err=b.leads.takeInWork(number,adminID)
	case "ack":l,changed,err=b.leads.acceptUpdate(number,updateID,adminID)
	case "resolve":l,changed,err=b.leads.resolveCase(number,adminID)
	default:return MiniCase{},errors.New("неизвестное действие")
	}
	if err!=nil{return MiniCase{},err}
	if changed{b.queueLeadDelivery(number)}
	return miniCase(l),nil
}
