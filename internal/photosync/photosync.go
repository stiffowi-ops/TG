package photosync

// Yandex Disk galleries are stored separately from the curated catalog.
// Temporary cloud links are never exposed to clients.
import(
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "path"
 "path/filepath"
 "regexp"
 "sort"
 "strconv"
 "strings"
 "sync"
 "sync/atomic"
 "time"
 "unicode"

 "locationbot/internal/catalog"
)

const DefaultPublicURL="https://disk.yandex.ru/d/TpiMfBy9WROTGg"
const maxImageBytes int64=18<<20
var ErrBusy=errors.New("photo sync already running")
var suffixNumber=regexp.MustCompile(`(?i)(?:[\s _.–—-]+|\s*\()(?:(?:фото|фотка|photo|image|img|ориентир)[\s_.–—-]*)?0*([1-9][0-9]?)\s*\)?package photosync

// Yandex Disk galleries are stored separately from the curated catalog.
// Temporary cloud links are never exposed to clients.
import(
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "path"
 "path/filepath"
 "regexp"
 "sort"
 "strconv"
 "strings"
 "sync"
 "sync/atomic"
 "time"
 "unicode"

 "locationbot/internal/catalog"
)

const DefaultPublicURL="https://disk.yandex.ru/d/TpiMfBy9WROTGg"
const maxImageBytes int64=18<<20
var ErrBusy=errors.New("photo sync already running")
)
var prefixNumber=regexp.MustCompile(`^\s*[0-9]{2,4}\s*[-_.–— ]+\s*`)
var photoOnly=regexp.MustCompile(`(?i)^(?:фото|фотка|photo|image|img|ориентир)?[\s_-]*0*([1-9][0-9]?)$`)

type diskItem struct{Name string `json:"name"`;Type string `json:"type"`;Modified string `json:"modified"`;Size int64 `json:"size"`}
type diskPage struct{Type string `json:"type"`; Embedded struct{Items []diskItem `json:"items"`} `json:"_embedded"`}
type downloadLink struct{Href string `json:"href"`}
type Photo struct{Source string `json:"source"`;Modified string `json:"modified"`;File string `json:"file"`}
type Manifest struct{Groups map[string][]Photo `json:"groups"`;Updated time.Time `json:"updated"`}
type Report struct{
 Enabled bool `json:"enabled"`;Running bool `json:"running"`
 LastAttempt time.Time `json:"lastAttempt,omitempty"`
 LastSuccess time.Time `json:"lastSuccess,omitempty"`
 MatchedLocations int `json:"matchedLocations"`
 ImagesAvailable int `json:"imagesAvailable"`
 Downloaded int `json:"downloaded"`
 Unchanged int `json:"unchanged"`
 Unmatched []string `json:"unmatched,omitempty"`
 Ambiguous []string `json:"ambiguous,omitempty"`
 Failures []string `json:"failures,omitempty"`
 Error string `json:"error,omitempty"`
}
type entry struct{item diskItem;rel,parent string;order int}
type Service struct{
 store *catalog.Store; publicURL,api,dir string
 client *http.Client
 running atomic.Bool
 mu sync.RWMutex
 manifest Manifest; report Report
}
func Canonical(value string)string{
 s:=strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value),"ё","е"))
 s=prefixNumber.ReplaceAllString(s,"")
 s=strings.ReplaceAll(s,"м.","метро ")
 var b strings.Builder
 for _,r:=range s {if unicode.IsLetter(r)||unicode.IsDigit(r){b.WriteRune(r)}else{b.WriteByte(' ')}}
 words:=strings.Fields(b.String())
 if len(words)>0&&(words[0]=="метро"||words[0]=="станция"){words=words[1:]}
 return strings.Join(words," ")
}
func imageExt(name string)bool{
 switch strings.ToLower(path.Ext(name)){case ".jpg",".jpeg",".png",".webp",".gif":return true}
 return false
}
func stemOrder(name string)(string,int){
 stem:=strings.TrimSuffix(name,path.Ext(name))
 stem=prefixNumber.ReplaceAllString(stem,"")
 number:=0
 if m:=suffixNumber.FindStringSubmatch(stem);len(m)==2{
  number,_=strconv.Atoi(m[1]);stem=suffixNumber.ReplaceAllString(stem,"")
 }
 return strings.TrimSpace(stem),number
}
func matchFiles(files []entry,locations []catalog.Location)(map[string][]entry,[]string,[]string){
 lookup:=map[string][]string{}
 for _,l:=range locations{key:=Canonical(l.Title);if key!=""{lookup[key]=append(lookup[key],l.ID)}}
 matched:=map[string][]entry{};unmatched:=[]string{};ambiguous:=[]string{}
 for _,file:=range files{
  if !imageExt(file.item.Name){continue}
  stem,order:=stemOrder(file.item.Name)
  key:=Canonical(stem)
  if parent:=Canonical(file.parent);parent!=""&&len(lookup[parent])>0{
   key=parent
   if m:=photoOnly.FindStringSubmatch(strings.TrimSuffix(file.item.Name,path.Ext(file.item.Name)));len(m)==2{order,_=strconv.Atoi(m[1])}
  }
  ids:=lookup[key]
  if len(ids)==0{if len(unmatched)<100{unmatched=append(unmatched,file.rel)};continue}
  if len(ids)>1{if len(ambiguous)<100{ambiguous=append(ambiguous,file.rel)};continue}
  file.order=order
  matched[ids[0]]=append(matched[ids[0]],file)
 }
 for id,group:=range matched{
  sort.SliceStable(group,func(i,j int)bool{
   left,right:=group[i].order,group[j].order
   if left==0{left=999};if right==0{right=999}
   if left!=right{return left<right}
   return strings.ToLower(group[i].rel)<strings.ToLower(group[j].rel)
  })
  if len(group)>3{group=group[:3]}
  matched[id]=group
 }
 return matched,unmatched,ambiguous
}
func New(store *catalog.Store,publicURL string)(*Service,error){
 if store==nil{return nil,errors.New("missing catalog")}
 if strings.TrimSpace(publicURL)==""{publicURL=DefaultPublicURL}
 u,err:=url.Parse(publicURL)
 if err!=nil||u.Scheme!="https"||(u.Hostname()!="disk.yandex.ru"&&u.Hostname()!="yadi.sk"){return nil,errors.New("invalid Yandex public share")}
 dir:=filepath.Join(store.DataDir(),"location_photos")
 if err=os.MkdirAll(dir,0755);err!=nil{return nil,err}
 s:=&Service{store:store,publicURL:publicURL,api:"https://cloud-api.yandex.net/v1/disk/public",dir:dir,client:&http.Client{Timeout:70*time.Second},manifest:Manifest{Groups:map[string][]Photo{}},report:Report{Enabled:true}}
 data,err:=os.ReadFile(filepath.Join(dir,"manifest.json"))
 if err==nil{
  var saved Manifest
  if err=json.Unmarshal(data,&saved);err!=nil{return nil,fmt.Errorf("photo manifest: %w",err)}
  if saved.Groups!=nil{s.manifest=saved;s.report.LastSuccess=saved.Updated}
 }else if !errors.Is(err,os.ErrNotExist){return nil,err}
 s.totals()
 return s,nil
}
func(s *Service) totals(){
 s.report.MatchedLocations=0;s.report.ImagesAvailable=0
 for _,group:=range s.manifest.Groups{
  if len(group)>0{s.report.MatchedLocations++}
  s.report.ImagesAvailable+=len(group)
 }
}
func(s *Service) Report()Report{
 s.mu.RLock();defer s.mu.RUnlock()
 r:=s.report;r.Running=s.running.Load()
 r.Unmatched=append([]string(nil),r.Unmatched...)
 r.Ambiguous=append([]string(nil),r.Ambiguous...)
 r.Failures=append([]string(nil),r.Failures...)
 return r
}
func(s *Service) Photos(id string)[]string{
 if !catalog.ValidID(id){return nil}
 s.mu.RLock();group:=append([]Photo(nil),s.manifest.Groups[id]...);s.mu.RUnlock()
 out:=make([]string,0,len(group))
 for _,p:=range group{
  if path.Base(p.File)!=p.File||!strings.HasPrefix(p.File,id+"-"){continue}
  if stat,err:=os.Stat(filepath.Join(s.dir,p.File));err!=nil||!stat.Mode().IsRegular(){continue}
  hash:=sha256.Sum256([]byte(p.Source+"\x00"+p.Modified))
  out=append(out,"/location-photos/"+url.PathEscape(p.File)+"?v="+hex.EncodeToString(hash[:4]))
 }
 return out
}
func(s *Service) ServePhoto(w http.ResponseWriter,r *http.Request){
 if r.Method!="GET"&&r.Method!="HEAD"{w.WriteHeader(405);return}
 file:=strings.TrimPrefix(r.URL.Path,"/location-photos/")
 if file==""||path.Base(file)!=file||strings.Contains(file,"\\"){http.NotFound(w,r);return}
 found:=false
 s.mu.RLock()
 for _,group:=range s.manifest.Groups{for _,p:=range group{if p.File==file{found=true;break}};if found{break}}
 s.mu.RUnlock()
 if !found{http.NotFound(w,r);return}
 w.Header().Set("Cache-Control","public,max-age=3600")
 w.Header().Set("X-Content-Type-Options","nosniff")
 http.ServeFile(w,r,filepath.Join(s.dir,file))
}
func(s *Service) endpoint(op,rel string,offset int)string{
 u,_:=url.Parse(s.api+op);q:=u.Query();q.Set("public_key",s.publicURL)
 if rel!=""{q.Set("path","/"+strings.TrimPrefix(rel,"/"))}
 if op=="/resources"{q.Set("offset",strconv.Itoa(offset));q.Set("limit","200")}
 u.RawQuery=q.Encode()
 return u.String()
}
func(s *Service) jsonGet(ctx context.Context,link string,dst any)error{
 req,err:=http.NewRequestWithContext(ctx,"GET",link,nil)
 if err!=nil{return err}
 res,err:=s.client.Do(req)
 if err!=nil{return err}
 defer res.Body.Close()
 if res.StatusCode!=200{b,_:=io.ReadAll(io.LimitReader(res.Body,200));return fmt.Errorf("Yandex HTTP %d: %s",res.StatusCode,strings.TrimSpace(string(b)))}
 return json.NewDecoder(io.LimitReader(res.Body,5<<20)).Decode(dst)
}
func(s *Service) list(ctx context.Context)([]entry,error){
 queue:=[]string{""};seen:=map[string]bool{};found:=[]entry{}
 for len(queue)>0{
  dir:=queue[0];queue=queue[1:]
  if seen[dir]{continue};seen[dir]=true
  if len(seen)>2000{return nil,errors.New("too many Yandex folders")}
  for offset:=0;offset<10000;offset+=200{
   var page diskPage
   if err:=s.jsonGet(ctx,s.endpoint("/resources",dir,offset),&page);err!=nil{return nil,fmt.Errorf("list %s: %w",dir,err)}
   if offset==0&&page.Type=="file"{return nil,errors.New("public share must be a folder")}
   for _,item:=range page.Embedded.Items{
    if item.Name==""||item.Name=="."||item.Name==".."||strings.Contains(item.Name,"/"){continue}
    rel:=strings.TrimPrefix(path.Join(dir,item.Name),"/")
    if item.Type=="dir"{queue=append(queue,rel)}
    if item.Type=="file"&&imageExt(item.Name){
     parent:="";if dir!=""{parent=path.Base(dir)}
     found=append(found,entry{item:item,rel:rel,parent:parent})
     if len(found)>10000{return nil,errors.New("too many Yandex photos")}
    }
   }
   if len(page.Embedded.Items)<200{break}
  }
 }
 if len(found)==0{return nil,errors.New("Yandex folder has no supported images")}
 return found,nil
}
func allowedHost(host string)bool{
 return host=="downloader.disk.yandex.ru"||strings.HasSuffix(host,".disk.yandex.ru")||strings.HasSuffix(host,".disk.yandex.net")
}
func(s *Service) download(ctx context.Context,file entry,id string,slot int)(Photo,error){
 var link downloadLink
 if err:=s.jsonGet(ctx,s.endpoint("/resources/download",file.rel,0),&link);err!=nil{return Photo{},err}
 u,err:=url.Parse(link.Href)
 if err!=nil||u.Scheme!="https"||u.User!=nil||!allowedHost(u.Hostname()){return Photo{},errors.New("unexpected Yandex download host")}
 req,err:=http.NewRequestWithContext(ctx,"GET",u.String(),nil);if err!=nil{return Photo{},err}
 res,err:=s.client.Do(req);if err!=nil{return Photo{},err}
 defer res.Body.Close()
 if res.StatusCode!=200{return Photo{},fmt.Errorf("photo HTTP %d",res.StatusCode)}
 if res.ContentLength>maxImageBytes{return Photo{},errors.New("photo larger than 18 MiB")}
 temp,err:=os.CreateTemp(s.dir,".photo-*");if err!=nil{return Photo{},err}
 defer os.Remove(temp.Name())
 size,err:=io.Copy(temp,io.LimitReader(res.Body,maxImageBytes+1))
 if err!=nil{temp.Close();return Photo{},err}
 if size>maxImageBytes{temp.Close();return Photo{},errors.New("photo larger than 18 MiB")}
 if _,err=temp.Seek(0,io.SeekStart);err!=nil{temp.Close();return Photo{},err}
 sniff:=make([]byte,512);n,_:=temp.Read(sniff)
 ext:=""
 switch http.DetectContentType(sniff[:n]){
 case "image/jpeg":ext=".jpg"
 case "image/png":ext=".png"
 case "image/webp":ext=".webp"
 case "image/gif":ext=".gif"
 default:temp.Close();return Photo{},errors.New("unsupported image format")}
 if err=temp.Chmod(0644);err!=nil{temp.Close();return Photo{},err}
 if err=temp.Close();err!=nil{return Photo{},err}
 filename:=fmt.Sprintf("%s-%d%s",id,slot,ext)
 if err=os.Rename(temp.Name(),filepath.Join(s.dir,filename));err!=nil{return Photo{},err}
 return Photo{Source:file.rel,Modified:file.item.Modified,File:filename},nil
}
func(s *Service) save(m Manifest)error{
 data,err:=json.MarshalIndent(m,"","  ");if err!=nil{return err}
 file,err:=os.CreateTemp(s.dir,".manifest-*");if err!=nil{return err}
 defer os.Remove(file.Name())
 if _,err=file.Write(data);err!=nil{file.Close();return err}
 if err=file.Chmod(0644);err!=nil{file.Close();return err}
 if err=file.Close();err!=nil{return err}
 return os.Rename(file.Name(),filepath.Join(s.dir,"manifest.json"))
}
func(s *Service) Run(ctx context.Context)(Report,error){
 if !s.running.CompareAndSwap(false,true){return s.Report(),ErrBusy}
 defer s.running.Store(false)
 s.mu.Lock();s.report.LastAttempt=time.Now().UTC();s.report.Error="";s.mu.Unlock()
 files,err:=s.list(ctx)
 if err!=nil{s.setError(err);return s.Report(),err}
 groups,unmatched,ambiguous:=matchFiles(files,s.store.List())
 if len(groups)==0{
  err=errors.New("no photo filenames match location names; photos preserved")
  s.setError(err);return s.Report(),err
 }
 s.mu.RLock();old:=s.manifest;s.mu.RUnlock()
 next:=Manifest{Groups:map[string][]Photo{},Updated:time.Now().UTC()}
 for id,g:=range old.Groups{next.Groups[id]=append([]Photo(nil),g...)}
 failed:=[]string{};downloaded,unchanged:=0,0
 ids:=make([]string,0,len(groups))
 for id:=range groups{ids=append(ids,id)}
 sort.Strings(ids)
 for _,id:=range ids{
  candidates:=groups[id];prev:=old.Groups[id];out:=[]Photo{}
  for index,file:=range candidates{
   var previous Photo;if index<len(prev){previous=prev[index]}
   if previous.Source==file.rel&&previous.Modified==file.item.Modified&&previous.File!=""{
    if info,e:=os.Stat(filepath.Join(s.dir,previous.File));e==nil&&info.Mode().IsRegular(){
     out=append(out,previous);unchanged++;continue
    }
   }
   if file.item.Size>maxImageBytes{
    if len(failed)<50{failed=append(failed,file.rel+": over 18 MiB")}
    if previous.File!=""{out=append(out,previous)}
    continue
   }
   photo,e:=s.download(ctx,file,id,index+1)
   if e!=nil{
    if len(failed)<50{failed=append(failed,file.rel+": "+e.Error())}
    if previous.File!=""{out=append(out,previous)}
    continue
   }
   downloaded++;out=append(out,photo)
  }
  if len(out)>0{next.Groups[id]=out}
 }
 if err=s.save(next);err!=nil{s.setError(err);return s.Report(),err}
 s.mu.Lock();s.manifest=next
 s.report.LastSuccess=next.Updated;s.report.Downloaded=downloaded;s.report.Unchanged=unchanged
 s.report.Unmatched=unmatched;s.report.Ambiguous=ambiguous;s.report.Failures=failed;s.report.Error=""
 s.totals();s.mu.Unlock()
 return s.Report(),nil
}
func(s *Service) setError(err error){s.mu.Lock();s.report.Error=err.Error();s.mu.Unlock()}
func(s *Service) Start(ctx context.Context){
 go func(){
  attempt,cancel:=context.WithTimeout(ctx,45*time.Minute)
  _,_=s.Run(attempt);cancel()
  ticker:=time.NewTicker(12*time.Hour);defer ticker.Stop()
  for{select{
   case <-ctx.Done():return
   case <-ticker.C:
    c,cancel:=context.WithTimeout(ctx,45*time.Minute);_,_=s.Run(c);cancel()
  }}
 }()
}
func(s *Service) Trigger()bool{
 if s.running.Load(){return false}
 go func(){ctx,cancel:=context.WithTimeout(context.Background(),45*time.Minute);defer cancel();_,_=s.Run(ctx)}()
 return true
}
