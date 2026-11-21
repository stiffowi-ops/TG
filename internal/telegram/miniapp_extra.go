package telegram

import (
	"errors"
	"time"

	"locationbot/internal/analytics"
)

type MiniMetric struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Views int `json:"views"`
	Unique int `json:"unique"`
	Additions int `json:"additions"`
	Saved int `json:"saved"`
	Leads int `json:"leads"`
}

type MiniVisitor struct {
	UserID int64 `json:"userId"`
	Name string `json:"name"`
	Username string `json:"username"`
	FirstAt time.Time `json:"firstAt"`
	LastAt time.Time `json:"lastAt"`
	Visits int `json:"visits"`
	HasLead bool `json:"hasLead"`
	Favorites []string `json:"favorites"`
}

func (b *Bot) MiniObserve(userID int64,username,firstName,locationID string) {
	if b==nil || userID<=0 || b.IsMiniAdmin(userID) {return}
	if b.visitors!=nil {
		_ = b.visitors.record(user{ID:userID,Username:username,FirstName:firstName},"Mini App",false,time.Now())
	}
	if locationID!="" {
		if _,ok:=b.store.Find(locationID);ok {b.recordLocationAnalytics(userID,locationID,analytics.View)}
	}
}

func (b *Bot) MiniAdminAnalytics(adminID int64, metric string,days int) ([]MiniMetric,error) {
	if !b.IsMiniAdmin(adminID) {return nil,errors.New("доступ запрещён")}
	if b.metrics==nil {return []MiniMetric{},nil}
	if days!=7&&days!=30&&days!=0{days=30}
	switch metric {case "views","unique","added","saved","leads":default:metric="views"}
	locs:=b.store.List()
	ids:=make([]string,0,len(locs))
	names:=map[string]string{}
	for _,l:=range locs {ids=append(ids,l.ID);names[l.ID]=publicText(l.Title)}
	counts:=b.metrics.AllRankings(time.Now(),days,metric,ids,b.adminCurrentFavorites(),b.adminLeadDates())
	out:=make([]MiniMetric,0,len(counts))
	for _,c:=range counts {
		out=append(out,MiniMetric{ID:c.LocationID,Name:names[c.LocationID],Views:c.Views,Unique:c.UniqueViews,Additions:c.Additions,Saved:c.CurrentFavorites,Leads:c.Leads})
	}
	return out,nil
}

func (b *Bot) MiniAdminVisitors(adminID int64) ([]MiniVisitor,error) {
	if !b.IsMiniAdmin(adminID){return nil,errors.New("доступ запрещён")}
	out:=[]MiniVisitor{}
	if b.visitors==nil {return out,nil}
	for _,p:=range b.visitors.snapshot(){
		if p.Hidden {continue}
		visits:=0
		for _,n:=range p.Daily {visits+=n}
		out=append(out,MiniVisitor{UserID:p.UserID,Name:p.FirstName+" "+p.LastName,Username:p.Username,FirstAt:p.FirstAt,LastAt:p.LastAt,Visits:visits,HasLead:p.HasLead,Favorites:b.MiniFavorites(p.UserID)})
		if len(out)>=500{break}
	}
	return out,nil
}
