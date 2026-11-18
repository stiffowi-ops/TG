package adminbackup

import (
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strconv"
 "time"
)

// DeletionLedger retains only the minimal ID and deletion time needed to keep
// permanently erased records from reappearing after old backup restoration.
// IDs remain in this technical ledger until explicitly purged with a separate
// data-retention process.
const DeletionFile = "deletion_ledger.json"
type DeletionLedger struct {
 Leads map[string]time.Time `json:"leads"`
 Visitors map[string]time.Time `json:"visitors"`
}
func blankLedger() DeletionLedger {
 return DeletionLedger{Leads:map[string]time.Time{},Visitors:map[string]time.Time{}}
}
func ParseDeletionLedger(raw []byte) (DeletionLedger,error) {
 if len(raw)==0 {return blankLedger(),nil}
 var out DeletionLedger
 if err:=json.Unmarshal(raw,&out);err!=nil{return out,err}
 if out.Leads==nil{out.Leads=map[string]time.Time{}}
 if out.Visitors==nil{out.Visitors=map[string]time.Time{}}
 for key,when:=range out.Leads {
  id,err:=strconv.ParseInt(key,10,64)
  if err!=nil||id<=0||when.IsZero(){return out,fmt.Errorf("invalid deleted lead marker: %s",key)}
 }
 for key,when:=range out.Visitors {
  id,err:=strconv.ParseInt(key,10,64)
  if err!=nil||id<=0||when.IsZero(){return out,fmt.Errorf("invalid deleted visitor marker: %s",key)}
 }
 return out,nil
}
func ReadDeletionLedger(root string) (DeletionLedger,error) {
 raw,err:=os.ReadFile(filepath.Join(root,DeletionFile))
 if errors.Is(err,os.ErrNotExist){return blankLedger(),nil}
 if err!=nil{return DeletionLedger{},err}
 return ParseDeletionLedger(raw)
}
func mergeDeletionLedgers(archive,current []byte)(DeletionLedger,error) {
 old,err:=ParseDeletionLedger(archive);if err!=nil{return old,err}
 now,err:=ParseDeletionLedger(current);if err!=nil{return now,err}
 for id,at:=range old.Leads {
  if t,ok:=now.Leads[id];!ok||at.After(t){now.Leads[id]=at}
 }
 for id,at:=range old.Visitors {
  if t,ok:=now.Visitors[id];!ok||at.After(t){now.Visitors[id]=at}
 }
 return now,nil
}
func (l DeletionLedger) Marshal() ([]byte,error){return json.MarshalIndent(l,"","  ")}
func MarkDeleted(root string,leadNums,visitorIDs []int64) error {
 ledger,err:=ReadDeletionLedger(root);if err!=nil{return err}
 now:=time.Now().UTC()
 for _,id:=range leadNums {
  if id<=0{return errors.New("invalid deleted lead ID")}
  ledger.Leads[strconv.FormatInt(id,10)]=now
 }
 for _,id:=range visitorIDs {
  if id<=0{return errors.New("invalid deleted visitor ID")}
  ledger.Visitors[strconv.FormatInt(id,10)]=now
 }
 raw,err:=ledger.Marshal();if err!=nil{return err}
 return atomicWrite(filepath.Join(root,DeletionFile),raw)
}
func FilterDeletedLeads(raw []byte,ledger DeletionLedger)([]byte,error){
 state,err:=parseLeads(raw);if err!=nil{return nil,err}
 for key,entry:=range state.Submitted {
  identity,e:=decodeLead(entry);if e!=nil{return nil,e}
  if _,deleted:=ledger.Leads[strconv.FormatInt(identity.Number,10)];deleted{delete(state.Submitted,key);delete(state.Drafts,key)}
 }
 retained:=make([]json.RawMessage,0,len(state.History))
 for _,entry:=range state.History {
  identity,e:=decodeLead(entry);if e!=nil{return nil,e}
  if _,deleted:=ledger.Leads[strconv.FormatInt(identity.Number,10)];!deleted {retained=append(retained,entry)}
 }
 state.History=retained
 return json.MarshalIndent(state,"","  ")
}
func FilterDeletedVisitors(raw []byte,ledger DeletionLedger)([]byte,error){
 records,err:=parseVisitors(raw);if err!=nil{return nil,err}
 for id:=range ledger.Visitors{delete(records,id)}
 return json.MarshalIndent(records,"","  ")
}
