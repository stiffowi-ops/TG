package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"locationbot/internal/catalog"
	"locationbot/internal/adminbackup"
	"path/filepath"
	"locationbot/internal/telegram"
)

type telegramUser struct {
	ID int64 `json:"id"`
	Username string `json:"username"`
	FirstName string `json:"first_name"`
}

func verifyTelegramInitData(raw, token string, now time.Time) (telegramUser,error) {
	var user telegramUser
	if token==""||len(raw)==0||len(raw)>8192{return user,errors.New("Telegram authentication required")}
	values,err:=url.ParseQuery(raw)
	if err!=nil{return user,errors.New("invalid Telegram parameters")}
	hashes:=values["hash"]
	if len(hashes)!=1||len(hashes[0])!=64{return user,errors.New("invalid Telegram signature")}
	expectedHash,err:=hex.DecodeString(hashes[0])
	if err!=nil{return user,errors.New("invalid Telegram signature")}
	delete(values,"hash")
	keys:=make([]string,0,len(values))
	for key,parts:=range values {
		if len(parts)!=1||key=="" {return user,errors.New("invalid Telegram parameters")}
		keys=append(keys,key)
	}
	sort.Strings(keys)
	lines:=make([]string,0,len(keys))
	for _,key:=range keys {lines=append(lines,key+"="+values.Get(key))}
	secretMAC:=hmac.New(sha256.New,[]byte("WebAppData"))
	secretMAC.Write([]byte(token))
	sigMAC:=hmac.New(sha256.New,secretMAC.Sum(nil))
	sigMAC.Write([]byte(strings.Join(lines,"\n")))
	if !hmac.Equal(sigMAC.Sum(nil),expectedHash) {return user,errors.New("invalid Telegram signature")}
	seconds,err:=strconv.ParseInt(values.Get("auth_date"),10,64)
	if err!=nil {return user,errors.New("invalid Telegram date")}
	at:=time.Unix(seconds,0)
	if at.After(now.Add(time.Minute))||now.Sub(at)>24*time.Hour {return user,errors.New("Telegram session has expired")}
	if err=json.Unmarshal([]byte(values.Get("user")),&user);err!=nil||user.ID<=0 {
		return telegramUser{},errors.New("invalid Telegram user")
	}
	return user,nil
}

type appService struct {
	store *catalog.Store
	bot *telegram.Bot
	token string
}

func jsonResponse(w http.ResponseWriter,status int,result any) {
	w.Header().Set("Content-Type","application/json; charset=utf-8")
	w.Header().Set("Cache-Control","no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}

func (s *appService) authenticated(w http.ResponseWriter,r *http.Request)(telegramUser,bool){
	if s.bot==nil {
		jsonResponse(w,http.StatusServiceUnavailable,map[string]string{"error":"Telegram bot is not configured"})
		return telegramUser{},false
	}
	raw:=r.Header.Get("X-Telegram-Init-Data")
	u,err:=verifyTelegramInitData(raw,s.token,time.Now())
	if err!=nil {
		jsonResponse(w,http.StatusUnauthorized,map[string]string{"error":err.Error()})
		return telegramUser{},false
	}
	return u,true
}

func readBody(r *http.Request,target any) error {
	r.Body=http.MaxBytesReader(nil,r.Body,16384)
	dec:=json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

func (s *appService) handler(webDist string) http.Handler {
	original:=routes(webDist)
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		w.Header().Set("X-Content-Type-Options","nosniff")
		if r.Method=="GET"&&r.URL.Path=="/api/locations"&&s.store!=nil {
			items:=s.store.List()
			type loc struct {
				ID string `json:"id"`
				Title string `json:"title"`
				Category string `json:"category"`
				Area string `json:"area"`
				Latitude *float64 `json:"latitude,omitempty"`
				Longitude *float64 `json:"longitude,omitempty"`
				ImageURL string `json:"imageUrl,omitempty"`
				MapURL string `json:"mapUrl,omitempty"`
				Description string `json:"description,omitempty"`
				Landmarks string `json:"landmarks,omitempty"`
				LineStatus string `json:"lineStatus,omitempty"`
				Traffic string `json:"traffic,omitempty"`
				Payers string `json:"payers,omitempty"`
				Opportunity string `json:"opportunity,omitempty"`
				Conclusion string `json:"conclusion,omitempty"`
			}
			result:=make([]loc,0,len(items))
			for _,l:=range items {
				result=append(result,loc{
					ID:l.ID,Title:l.Title,Category:l.Category,Area:l.Category,
					Latitude:l.Latitude,Longitude:l.Longitude,ImageURL:l.ImageURL,
					MapURL:l.MapURL,Description:l.Description,Landmarks:l.Landmarks,
					LineStatus:l.LineStatus,Traffic:l.Traffic,Payers:l.Payers,
					Opportunity:l.Opportunity,Conclusion:l.Conclusion,
				})
			}
			jsonResponse(w,200,map[string]any{"locations":result,"demo":false})
			return
		}
		if r.Method=="GET" && strings.HasPrefix(r.URL.Path,"/media/") && s.store!=nil {
   fileName:=strings.TrimPrefix(r.URL.Path,"/media/")
   ext:=filepath.Ext(fileName)
   id:=strings.TrimSuffix(fileName,ext)
   if !catalog.ValidID(id)||!catalog.ValidExt(ext)||filepath.Base(fileName)!=fileName {http.NotFound(w,r);return}
   w.Header().Set("Cache-Control","public, max-age=3600")
   http.ServeFile(w,r,filepath.Join(s.store.DataDir(),"images",fileName))
   return
  }
  if !strings.HasPrefix(r.URL.Path,"/api/") || r.URL.Path=="/api/health" {
			original.ServeHTTP(w,r);return
		}
		u,ok:=s.authenticated(w,r)
		if !ok{return}
		switch {
		case r.Method=="POST"&&r.URL.Path=="/api/locations/view":
   var input struct{ID string `json:"id"`}
   if err:=readBody(r,&input);err!=nil{jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
   if _,exists:=s.store.Find(input.ID);!exists{jsonResponse(w,404,map[string]string{"error":"location not found"});return}
   s.bot.MiniObserve(u.ID,u.Username,u.FirstName,input.ID)
   jsonResponse(w,200,map[string]bool{"ok":true})
  case r.Method=="GET"&&r.URL.Path=="/api/me":
   s.bot.MiniObserve(u.ID,u.Username,u.FirstName,"")
			jsonResponse(w,200,map[string]any{
				"user":map[string]any{"id":u.ID,"username":u.Username,"firstName":u.FirstName},
				"admin":s.bot.IsMiniAdmin(u.ID),
				"favorites":s.bot.MiniFavorites(u.ID),
				"cases":s.bot.MiniMyCases(u.ID),
			})
		case r.Method=="POST"&&r.URL.Path=="/api/favorites/toggle":
			var input struct{ID string `json:"id"`}
			if err:=readBody(r,&input);err!=nil {jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
			ids,err:=s.bot.MiniToggleFavorite(u.ID,input.ID)
			if err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
			jsonResponse(w,200,map[string]any{"favorites":ids})
		case r.Method=="POST"&&r.URL.Path=="/api/cases":
			var input telegram.MiniNewCase
			if err:=readBody(r,&input);err!=nil {jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
			c,err:=s.bot.MiniCreateCase(u.ID,u.Username,input)
			if err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
			jsonResponse(w,201,map[string]any{"case":c})
		case r.Method=="POST"&&r.URL.Path=="/api/cases/supplement":
			var input struct {
				Number int64 `json:"number"`
				Telegram telegram.MiniSupplement `json:"supplement"`
			}
			if err:=readBody(r,&input);err!=nil{jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
			c,err:=s.bot.MiniAddSupplement(u.ID,input.Number,input.Telegram)
			if err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
			jsonResponse(w,200,map[string]any{"case":c})
		case r.Method=="GET"&&r.URL.Path=="/api/admin/analytics":
   if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
   days,_:=strconv.Atoi(r.URL.Query().Get("days"))
   metrics,err:=s.bot.MiniAdminAnalytics(u.ID,r.URL.Query().Get("metric"),days)
   if err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
   jsonResponse(w,200,map[string]any{"metrics":metrics})
  case r.Method=="GET"&&r.URL.Path=="/api/admin/visitors":
   if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
   users,err:=s.bot.MiniAdminVisitors(u.ID)
   if err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
   jsonResponse(w,200,map[string]any{"visitors":users})
  case r.Method=="GET"&&r.URL.Path=="/api/admin/location":
   if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
   l,ok:=s.store.Find(r.URL.Query().Get("id"));if !ok{jsonResponse(w,404,map[string]string{"error":"location not found"});return}
   rev:=catalog.Revision(l)
   jsonResponse(w,200,map[string]any{"revision":rev,"hash":catalog.RevisionHash(rev)})
  case r.Method=="POST"&&r.URL.Path=="/api/admin/location":
   if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
   var input struct {
    ID string `json:"id"`
    Hash string `json:"hash"`
    Revision catalog.CardRevision `json:"revision"`
   }
   if err:=readBody(r,&input);err!=nil{jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
   if input.Hash==""{jsonResponse(w,400,map[string]string{"error":"hash required"});return}
   if err:=s.store.AdminEdit(input.ID,input.Hash,input.Revision);err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
   jsonResponse(w,200,map[string]bool{"ok":true})
  case r.Method=="POST"&&r.URL.Path=="/api/admin/location/create":
   if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
   var input catalog.CardRevision
   if err:=readBody(r,&input);err!=nil{jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
   id,err:=catalog.NewAdminLocationID()
   if err!=nil{jsonResponse(w,500,map[string]string{"error":"could not generate ID"});return}
   loc:=catalog.Location{ID:id,Title:input.Title,Category:input.Category,Description:input.Description,Rating:input.Rating,Latitude:input.Latitude,Longitude:input.Longitude,LineStatus:input.LineStatus,Landmarks:input.Landmarks,MapURL:input.MapURL,Sections:input.Sections}
   if err:=s.store.CreateAdminLocation(loc);err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
   jsonResponse(w,201,map[string]string{"id":id})
  case r.Method=="GET"&&r.URL.Path=="/api/admin/backup":
   if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
   data,err:=adminbackup.Build(s.store.DataDir(),s.store.List())
   if err!=nil{jsonResponse(w,500,map[string]string{"error":"backup unavailable"});return}
   w.Header().Set("Content-Type","application/zip")
   w.Header().Set("Cache-Control","no-store")
   w.Header().Set("Content-Disposition",`attachment; filename="miniapp-backup.zip"`)
   w.WriteHeader(200);_,_=w.Write(data)
  case r.Method=="GET"&&r.URL.Path=="/api/admin/cases":
			if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
			items,err:=s.bot.MiniAdminCases(u.ID)
			if err!=nil{jsonResponse(w,403,map[string]string{"error":err.Error()});return}
			jsonResponse(w,200,map[string]any{"cases":items})
		case r.Method=="POST"&&r.URL.Path=="/api/admin/action":
			if !s.bot.IsMiniAdmin(u.ID){jsonResponse(w,403,map[string]string{"error":"access denied"});return}
			var input struct {
				Number int64 `json:"number"`
				Action string `json:"action"`
				UpdateID int `json:"updateId"`
			}
			if err:=readBody(r,&input);err!=nil{jsonResponse(w,400,map[string]string{"error":"invalid request"});return}
			c,err:=s.bot.MiniAdminAction(u.ID,input.Number,input.Action,input.UpdateID)
			if err!=nil{jsonResponse(w,400,map[string]string{"error":err.Error()});return}
			jsonResponse(w,200,map[string]any{"case":c})
		default:
			jsonResponse(w,404,map[string]string{"error":fmt.Sprintf("unknown endpoint %s",r.URL.Path)})
		}
	})
}
