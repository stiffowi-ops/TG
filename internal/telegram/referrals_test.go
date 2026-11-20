package telegram

import (
 "path/filepath"
 "strings"
 "testing"
)

func TestReferralFlowAndIdempotentAward(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"leads.json")
 s,err:=openLeadStore(path);if err!=nil{t.Fatal(err)}
 b:=&Bot{leads:s,adminIDs:map[int64]bool{90:true}}
 code,err:=s.newReferralCode(101);if err!=nil{t.Fatal(err)}
 if _,err=b.MiniValidateReferral(101,code);err==nil{t.Fatal("self referral accepted")}
 if _,err=b.MiniValidateReferral(202,"ref_invalid");err==nil{t.Fatal("invalid referral accepted")}
 link:="https://t.me/LocationBot?startapp="+code
 v,err:=b.MiniValidateReferral(202,link);if err!=nil||v.Code!=code{t.Fatalf("link validation %v %+v",err,v)}
 captured,err:=b.MiniCaptureReferral(202,code);if err!=nil||captured!=code{t.Fatal("cannot capture referral",err)}
 if err:=s.putDraft(202,leadDraft{Source:"menu",Stage:"review",Kind:"existing",Name:"Приглашённый",Phone:"+79065288935",LocationID:"001"});err!=nil{t.Fatal(err)}
 lead,already,err:=s.submit(202,"test","Метро");if err!=nil||already{t.Fatalf("submit: %v %v",err,already)}
 if lead.ReferrerID!=101||lead.ReferralStatus!="pending"||lead.ReferralCode!=code{t.Fatalf("missing attribution: %+v",lead)}
 if _,err:=b.MiniValidateReferral(202,code);err==nil{t.Fatal("repeat invitation accepted")}
 if _,_,err:=b.MiniAdminReferralAction(90,lead.Number,"award");err==nil{t.Fatal("award before take accepted")}
 if _,_,err:=b.MiniAdminReferralAction(202,lead.Number,"award");err==nil{t.Fatal("nonadmin awarded")}
 if _,_,err:=s.takeInWork(lead.Number,90);err!=nil{t.Fatal(err)}
 deferred,changed,err:=b.MiniAdminReferralAction(90,lead.Number,"defer")
 if err!=nil||!changed||deferred.Status!="deferred"{t.Fatalf("defer: %v %+v",err,deferred)}
 if _,ok:=b.MiniGetProfile(101);ok{t.Fatal("defer unexpectedly awarded")}
 queue,err:=b.MiniAdminReferralQueue(90);if err!=nil||len(queue)!=1{t.Fatalf("missing deferred queue: %+v %v",queue,err)}
 if _,err:=b.MiniAdminReferralQueue(202);err==nil{t.Fatal("nonadmin can view queue")}
 if _,_,err:=s.resolveCase(lead.Number,90);err!=nil{t.Fatal(err)}
 awarded,changed,err:=b.MiniAdminReferralAction(90,lead.Number,"award")
 if err!=nil||!changed||awarded.Percent!=10{t.Fatalf("award after close: %v %+v",err,awarded)}
 _,changed,err=b.MiniAdminReferralAction(90,lead.Number,"award")
 if err!=nil||changed{t.Fatal("duplicate award")}
 if _,_,err=b.MiniAdminReferralAction(90,lead.Number,"defer");err==nil{t.Fatal("awarded referral reversed")}
 got,ok:=b.MiniGetProfile(101);if !ok||got.DiscountPercent!=10||got.ReferralDiscountPercent!=10{t.Fatalf("missing client's award %+v %v",got,ok)}
 if len(s.referralNotices())!=1{t.Fatal("missing undelivered notice")}
 if err:=s.markReferralNotified(202,lead.Number);err!=nil{t.Fatal(err)}
 if len(s.referralNotices())!=0{t.Fatal("duplicate notice after receipt")}
 reopened,err:=openLeadStore(path);if err!=nil{t.Fatal(err)}
 persisted,_:=(&Bot{leads:reopened}).MiniGetProfile(101)
 if persisted.DiscountPercent!=10{t.Fatalf("reward lost after restart %+v",persisted)}
}

func TestReferralOptOutAndManualDiscountPrecedence(t *testing.T) {
 s,err:=openLeadStore(filepath.Join(t.TempDir(),"leads.json"));if err!=nil{t.Fatal(err)}
 b:=&Bot{leads:s,adminIDs:map[int64]bool{90:true}}
 code,err:=s.newReferralCode(10);if err!=nil{t.Fatal(err)}
 if _,err:=b.MiniCaptureReferral(20,code);err!=nil{t.Fatal(err)}
 if err:=b.MiniClearPendingReferral(20);err!=nil{t.Fatal(err)}
 if got:=b.MiniPendingReferral(20);got!=""{t.Fatal("opt out not persisted")}
 if err:=s.putDraft(20,leadDraft{Source:"menu",Stage:"review",Kind:"existing",Name:"Тест",Phone:"+79065288935",LocationID:"001"});err!=nil{t.Fatal(err)}
 submitted,_,err:=s.submit(20,"test","Метро");if err!=nil{t.Fatal(err)}
 if submitted.ReferrerID!=0{t.Fatal("opted-out referral attributed")}
 _,err=b.MiniSaveProfile(10,MiniProfile{Name:"Пригласивший",Phone:"+79065288935",Kind:"existing"})
 if err!=nil{t.Fatal(err)}
 _,err=b.MiniAdminSetDiscount(90,10,15);if err!=nil{t.Fatal(err)}
 s.mu.Lock()
 next:=s.cloneLocked()
 next.Referrals["33"]=referralRecord{InviteeID:33,InviterID:10,CaseNumber:99,Status:"awarded",Percent:10}
 err=s.saveLocked(next)
 s.mu.Unlock()
 if err!=nil{t.Fatal(err)}
 p,_:=b.MiniGetProfile(10)
 if p.DiscountPercent!=15||p.ManualDiscountPercent!=15||p.ReferralDiscountPercent!=10{t.Fatalf("wrong discount precedence: %+v",p)}
 if _,err=parseReferralCode(strings.Replace(code,"ref_","bad_",1));err==nil{t.Fatal("malformed code accepted")}
}
