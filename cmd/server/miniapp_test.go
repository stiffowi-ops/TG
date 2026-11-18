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
	user:=map[string]any{"id":12345,"first_name":"Test","username":"tester"}
	rawUser,_:=json.Marshal(user)
	fields:=url.Values{}
	fields.Set("auth_date",strconv.FormatInt(now.Unix(),10))
	fields.Set("user",string(rawUser))
	fields.Set("query_id","AAEx")
	keys:=[]string{"auth_date","query_id","user"}
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
