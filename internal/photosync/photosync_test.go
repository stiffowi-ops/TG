package photosync

import (
 "reflect"
 "testing"
 "locationbot/internal/catalog"
)
func TestCanonical(t *testing.T){
 cases:=map[string]string{
  "Метро Маяковская":"маяковская",
  "Маяковская":"маяковская",
  "М. Академическая":"академическая",
  "Метро Тёплый Стан":"теплый стан",
  "020 — Метро Академика Черенкова":"академика черенкова",
 }
 for input,want:=range cases {
  if got:=Canonical(input);got!=want{t.Errorf("%q => %q, want %q",input,got,want)}
 }
}
func TestMatchFlatAndFolders(t *testing.T){
 locations:=[]catalog.Location{
  {ID:"001",Title:"Метро Маяковская"},
  {ID:"020",Title:"Метро Академика Черенкова"},
 }
 files:=[]entry{
  {item:diskItem{Name:"Маяковская_3.jpg",Type:"file"},rel:"Маяковская_3.jpg"},
  {item:diskItem{Name:"Метро Маяковская 1.jpeg",Type:"file"},rel:"Метро Маяковская 1.jpeg"},
  {item:diskItem{Name:"Маяковская-2.png",Type:"file"},rel:"Маяковская-2.png"},
  {item:diskItem{Name:"1.jpg",Type:"file"},rel:"Метро Академика Черенкова/1.jpg",parent:"Метро Академика Черенкова"},
  {item:diskItem{Name:"2.jpg",Type:"file"},rel:"Метро Академика Черенкова/2.jpg",parent:"Метро Академика Черенкова"},
  {item:diskItem{Name:"Other.jpg",Type:"file"},rel:"Other.jpg"},
 }
 got,unmatched,ambiguous:=matchFiles(files,locations)
 if len(got["001"])!=3||len(got["020"])!=2{t.Fatalf("wrong grouping: %#v",got)}
 if !reflect.DeepEqual([]string{got["001"][0].rel,got["001"][1].rel,got["001"][2].rel},[]string{"Метро Маяковская 1.jpeg","Маяковская-2.png","Маяковская_3.jpg"}) {t.Fatalf("wrong ordering: %#v",got["001"])}
 if len(unmatched)!=1||len(ambiguous)!=0{t.Fatalf("unmatched=%v ambiguous=%v",unmatched,ambiguous)}
}
func TestNoAmbiguousPhotoBinding(t *testing.T){
 files:=[]entry{{item:diskItem{Name:"Маяковская 1.jpg",Type:"file"},rel:"Маяковская 1.jpg"}}
 locations:=[]catalog.Location{{ID:"001",Title:"Метро Маяковская"},{ID:"002",Title:"Маяковская"}}
 got,_,ambiguous:=matchFiles(files,locations)
 if len(got)!=0||len(ambiguous)!=1 {t.Fatalf("ambiguous match should be skipped")}
}

func TestRealYandexFolderFileNames(t *testing.T){
 locations:=[]catalog.Location{
  {ID:"001",Title:"Метро Остров Мечты"},
  {ID:"002",Title:"Метро Кленовый бульвар"},
 }
 files:=[]entry{
  {item:diskItem{Name:"001 Метро Остров Мечты — 1.jpg"},rel:"001 Метро Остров Мечты — 1.jpg"},
  {item:diskItem{Name:"001 Метро Остров Мечты — 2.jpg"},rel:"001 Метро Остров Мечты — 2.jpg"},
  {item:diskItem{Name:"001 Метро Остров Мечты — 3.jpg"},rel:"001 Метро Остров Мечты — 3.jpg"},
  {item:diskItem{Name:"002 Метро Кленовый бульвар — 1.jpg"},rel:"002 Метро Кленовый бульвар — 1.jpg"},
 }
 matched,unmatched,ambiguous:=matchFiles(files,locations)
 if len(matched["001"])!=3 || len(matched["002"])!=1 || len(unmatched)!=0 || len(ambiguous)!=0 {
  t.Fatalf("matching real files failed: matched=%v unmatched=%v ambiguous=%v",matched,unmatched,ambiguous)
 }
 for i,photo:=range matched["001"]{if photo.order!=i+1 {t.Fatalf("photo order %d at %d",photo.order,i)}}
}

func TestDuplicateStationNamesUseMatchingCatalogID(t *testing.T) {
 locations:=[]catalog.Location{{ID:"007",Title:"Метро Липецкая"},{ID:"061",Title:"Липецкая"},{ID:"001",Title:"Остров Мечты"}}
 files:=[]entry{
  {item:diskItem{Name:"007 Метро Липецкая — 2.jpg"},rel:"007 Метро Липецкая — 2.jpg"},
  {item:diskItem{Name:"1.jpg"},parent:"061 Липецкая",rel:"061 Липецкая/1.jpg"},
  {item:diskItem{Name:"Липецкая — 3.jpg"},rel:"Липецкая — 3.jpg"},
  {item:diskItem{Name:"001 Липецкая — 1.jpg"},rel:"001 Липецкая — 1.jpg"},
 }
 got,_,ambiguous:=matchFiles(files,locations)
 if len(got["007"])!=1||got["007"][0].rel!=files[0].rel||len(got["061"])!=1||got["061"][0].rel!=files[1].rel||len(ambiguous)!=2 {
  t.Fatalf("matched=%v ambiguous=%v",got,ambiguous)
 }
}
