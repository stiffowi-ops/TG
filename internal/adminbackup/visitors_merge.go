package adminbackup

import (
 "bytes"
 "encoding/json"
 "fmt"
 "strconv"
 "time"
)

// Kept independent of telegram package to avoid circular dependencies.
type visitorBackupRecord struct {
 UserID int64 `json:"user_id"`
 FirstName string `json:"first_name,omitempty"`
 LastName string `json:"last_name,omitempty"`
 Username string `json:"username,omitempty"`
 FirstAt time.Time `json:"first_at"`
 LastAt time.Time `json:"last_at"`
 LastStart time.Time `json:"last_start,omitempty"`
 FirstRoute []json.RawMessage `json:"first_route,omitempty"`
 FirstRouteClosed bool `json:"first_route_closed,omitempty"`
 Daily map[string]int `json:"daily,omitempty"`
 HasLead bool `json:"has_lead,omitempty"`
 Alerted bool `json:"alerted,omitempty"`
 Hidden bool `json:"hidden,omitempty"`
 AlertRecipients map[string]bool `json:"alert_recipients,omitempty"`
}
func parseVisitors(input []byte) (map[string]visitorBackupRecord,error) {
 if len(bytes.TrimSpace(input))==0 {return map[string]visitorBackupRecord{},nil}
 var result map[string]visitorBackupRecord
 if err:=json.Unmarshal(input,&result);err!=nil{return nil,err}
 if result==nil{return map[string]visitorBackupRecord{},nil}
 for key,p:=range result {
  if p.UserID<=0||strconv.FormatInt(p.UserID,10)!=key{return nil,fmt.Errorf("invalid visitor %s",key)}
  if p.FirstAt.IsZero() || p.LastAt.Before(p.FirstAt) {return nil,fmt.Errorf("invalid visitor timestamps %s",key)}
  for _,n:=range p.Daily{if n<0||n>1000000{return nil,fmt.Errorf("invalid visitor counts %s",key)}}
 }
 return result,nil
}
// Current runtime always wins for status, navigation, and contact fields.
// Earlier first sightings from a backup can be restored, but no old archive
// may reset HasLead/Alerted or re-trigger an already delivered alert.
func mergeVisitors(archived,current []byte)([]byte,error){
 older,err:=parseVisitors(archived);if err!=nil{return nil,err}
 newer,err:=parseVisitors(current);if err!=nil{return nil,err}
 for id,previous:=range older{
  now,ok:=newer[id]
  if !ok{newer[id]=previous;continue}
  if previous.FirstAt.Before(now.FirstAt){
   now.FirstAt=previous.FirstAt
   if len(previous.FirstRoute)>0{now.FirstRoute=previous.FirstRoute;now.FirstRouteClosed=previous.FirstRouteClosed}
  }
  if previous.LastAt.After(now.LastAt){now.LastAt=previous.LastAt}
  if previous.LastStart.After(now.LastStart){now.LastStart=previous.LastStart}
  if now.FirstName==""{now.FirstName=previous.FirstName}
  if now.LastName==""{now.LastName=previous.LastName}
  if now.Username==""{now.Username=previous.Username}
  if now.Daily==nil{now.Daily=map[string]int{}}
  for day,count:=range previous.Daily{if count>now.Daily[day]{now.Daily[day]=count}}
  now.HasLead=now.HasLead||previous.HasLead
  now.Alerted=now.Alerted||previous.Alerted
  now.FirstRouteClosed=now.FirstRouteClosed||previous.FirstRouteClosed
  if now.AlertRecipients==nil{now.AlertRecipients=map[string]bool{}}
  for admin,delivered:=range previous.AlertRecipients{if delivered{now.AlertRecipients[admin]=true}}
  newer[id]=now
 }
 return json.MarshalIndent(newer,"","  ")
}
