package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"os"
	"path/filepath"
	"locationbot/internal/catalog"
	"locationbot/internal/syncer"
	"locationbot/internal/telegram"
	"time"
)

func signedMiniInit(token string,now time.Time)string{
	return signedMiniLaunch(token,now,12345,"")
}

func signedMiniLaunch(token string,now time.Time,id int64,startParam string)string{
	user:=map[string]any{"id":id,"first_name":"Test","username":"tester"}
	rawUser,_:=json.Marshal(user)
	fields:=url.Values{}
	fields.Set("auth_date",strconv.FormatInt(now.Unix(),10))
	fields.Set("user",string(rawUser))
	fields.Set("query_id","AAEx")
	keys:=[]string{"auth_date","query_id","user"}
	if startParam!=""{
		fields.Set("start_param",startParam)
		keys=[]string{"auth_date","query_id","start_param","user"}
	}
	lines:=[]string{}
	for _,k:=range keys{lines=append(lines,k+"="+fields.Get(k))}
	secret:=hmac.New(sha256.New,[]byte("WebAppData"))
	secret.Write([]byte(token))
	mac:=hmac.New(sha256.New,secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines,string(rune(10)))))
	fields.Set("hash",hex.EncodeToString(mac.Sum(nil)))
	return fields.Encode()
}

func TestMiniTelegramSignedAuth(t *testing.T) {
	now:=time.Date(2026,11,18,12,0,0,0,time.UTC)
	token:="123456:secret-test-token"
	raw:=signedMiniInit(token,now)
	user,err:=verifyTelegramInitData(raw,token,now)
	if err!=nil||user.ID!=12345{t.Fatalf("signed init failed: %+v %v",user,err)}
	if _,err:=verifyTelegramInitData(strings.Replace(raw,"tester","hacker",1),token,now);err==nil{
		t.Fatal("tampered data must be rejected")
	}
	if _,err:=verifyTelegramInitData(raw,token,now.Add(25*time.Hour));err==nil{
		t.Fatal("expired data must be rejected")
	}
	if _,err:=verifyTelegramInitData(raw,"other:secret",now);err==nil{
		t.Fatal("different bot token must be rejected")
	}
}

func TestMiniAPIUnauthorizedWithoutBot(t *testing.T){
	handler:=(&appService{}).handler(t.TempDir())
	for _,path:=range []string{"/api/me","/api/admin/cases","/api/admin/analytics","/api/admin/backup"}{
		req:=httptest.NewRequest(http.MethodGet,path,nil)
		res:=httptest.NewRecorder()
		handler.ServeHTTP(res,req)
		if res.Code!=http.StatusServiceUnavailable {t.Errorf("%s: expected 503 got %d",path,res.Code)}
	}
}

func TestMiniAdminMustBeAuthorized(t *testing.T) {
	dir:=t.TempDir()
	seed:=filepath.Join(dir,"seed.json")
	if err:=os.WriteFile(seed,[]byte("[]"),0600);err!=nil{t.Fatal(err)}
	store,err:=catalog.NewWithBundled(dir,seed,filepath.Join(dir,"overrides.json"))
	if err!=nil {t.Fatal(err)}
	const token="123456:test"
	bot,err:=telegram.New(token,"",dir,store,syncer.New(store,""),"67890")
	if err!=nil{t.Fatal(err)}
	handler:=(&appService{store:store,bot:bot,token:token}).handler(dir)
	for _,test:=range []struct{name,data string;want int}{
		{"unsigned","",http.StatusUnauthorized},
		{"bad signature","auth_date=0&user=%7B%7D&hash=aaaa",http.StatusUnauthorized},
		{"authenticated non-admin",signedMiniInit(token,time.Now()),http.StatusForbidden},
	}{
		req:=httptest.NewRequest(http.MethodGet,"/api/admin/cases",nil)
		if test.data!=""{req.Header.Set("X-Telegram-Init-Data",test.data)}
		res:=httptest.NewRecorder();handler.ServeHTTP(res,req)
		if res.Code!=test.want{t.Errorf("%s: got %d expected %d",test.name,res.Code,test.want)}
	}
}

func TestSignedReferralBeforeProfileLoad(t *testing.T) {
	t.Setenv("REFERRAL_BOT_USERNAME", "LocationTestBot")
	t.Setenv("REFERRAL_MINIAPP_SHORT_NAME", "locations")
	dir:=t.TempDir()
	store,err:=catalog.NewWithBundled(dir,"../../config/seed.json",filepath.Join(dir,"overrides.json"))
	if err!=nil {t.Fatal(err)}
	const token="12345:local-test"
	bot,err:=telegram.New(token,"",dir,store,syncer.New(store,""),"90")
	if err!=nil {t.Fatal(err)}
	handler:=(&appService{store:store,bot:bot,token:token}).handler(dir)
	call:=func(method,path,data,body string)*httptest.ResponseRecorder{
		req:=httptest.NewRequest(method,path,strings.NewReader(body))
		req.Header.Set("X-Telegram-Init-Data",data)
		rec:=httptest.NewRecorder()
		handler.ServeHTTP(rec,req)
		return rec
	}
	now:=time.Now()
	reply:=call(http.MethodGet,"/api/referral",signedMiniLaunch(token,now,101,""),"")
	var invitation telegram.MiniReferralInfo
	if err=json.Unmarshal(reply.Body.Bytes(),&invitation);err!=nil||reply.Code!=200||invitation.Code==""||invitation.Link!="https://t.me/LocationTestBot/locations?startapp="+invitation.Code{t.Fatalf("invitation: %s",reply.Body.String())}
	for _,tc:=range []struct{id int64;skip bool;wantOwner int64}{{202,false,101},{203,true,0}}{
		data:=signedMiniLaunch(token,now,tc.id,invitation.Code)
		body,_:=json.Marshal(telegram.MiniNewCase{Kind:"existing",Name:"Test",Phone:"+79990000000",LocationID:store.List()[0].ID,SkipReferral:tc.skip})
		tampered:=strings.Replace(data,invitation.Code,"ref_AAAAAAAAAAAAAAAA",1)
		if got:=call(http.MethodPost,"/api/cases",tampered,string(body));got.Code!=401{t.Fatalf("unsigned referral accepted: %d",got.Code)}
		reply=call(http.MethodPost,"/api/cases",data,string(body))
		var result struct{Case telegram.MiniCase `json:"case"`}
		if err:=json.Unmarshal(reply.Body.Bytes(),&result);err!=nil||reply.Code!=201||result.Case.ReferrerID!=tc.wantOwner {t.Fatalf("submit: %d %s",reply.Code,reply.Body.String())}
		// Later /me calls with the same launch must not restore an already consumed invitation.
		call(http.MethodGet,"/api/me",data,"")
		if bot.MiniPendingReferral(tc.id)!=""{t.Fatal("invitation reattached after submit")}
	}
}
