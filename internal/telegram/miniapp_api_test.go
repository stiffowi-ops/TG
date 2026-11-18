package telegram

import (
	"encoding/json"
	"testing"
)

func TestMiniFavoritesSerializesEmptyArray(t *testing.T) {
	bot := &Bot{favorites:&favoriteStore{byUser:map[string][]string{}}}
	for _,tc := range []struct{name string; bot *Bot; userID int64}{
		{name:"new user",bot:bot,userID:123},
		{name:"no bot",bot:nil,userID:123},
		{name:"invalid id",bot:bot,userID:0},
	}{
		t.Run(tc.name,func(t *testing.T){
			items:=tc.bot.MiniFavorites(tc.userID)
			out,err:=json.Marshal(items)
			if err!=nil {t.Fatal(err)}
			if string(out)!="[]" {t.Fatalf("expected [], got %s",out)}
		})
	}
}
