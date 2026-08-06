package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// TestModelJSONContract 验证关键模型 JSON 字段与前端契约对齐。
func TestModelJSONContract(t *testing.T) {
	cases := []struct {
		name string
		val  any
		key  string
		want string
	}{
		{"alias", modelwarframe.Alias{En: "x"}, "en", `"x"`},
		{"nodes", modelwarframe.Nodes{UniqueName: "u"}, "uniqueName", `"u"`},
		{"orders", modelwarframe.OrdersItem{ID: "1"}, "bulkTradable", `false`},
		{"rivenTion", modelwarframe.RivenTion{IDs: 1}, "ids", `1`},
		{"missionSubscribe", modelwarframe.MissionSubscribe{SubBotUID: 5}, "subBotUid", `5`},
		{"stateTranslation", modelwarframe.StateTranslation{Type: 7}, "type", `"WARFRAMES"`},
		{"nightWave", modelwarframe.NightWave{Description: "x|COUNT|y", Required: 3}, "description", `"x3y"`},
		{"relicRewards", modelwarframe.RelicRewards{Rarity: 1}, "rarity", `"UNCOMMON"`},
		{"reward", modelwarframe.Reward{Rarity: 2}, "rarity", `"RARE"`},
		{"weapons", modelwarframe.Weapons{ProductCategory: 8}, "productCategory", `"Shotguns"`},
		{"warframes", modelwarframe.Warframes{SprintSpeed: 1}, "sprintSpeed", `1`},
		{"checkType", modelwarframe.MissionSubscribeUserCheckType{Subscribe: "ALERTS"}, "subscribe", `"ALERTS"`},
	}

	// 关联结构：RewardPool 嵌套 rewards，Relics 嵌套 relicRewards
	pool := modelwarframe.RewardPool{UniqueName: "p", Rewards: []modelwarframe.Reward{{ID: "r", Item: "i"}}}
	poolJSON, err := json.Marshal(pool)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(poolJSON), `"rewards"`) || !strings.Contains(string(poolJSON), `"item":"i"`) {
		t.Fatalf("reward pool association mismatch: %s", string(poolJSON))
	}
	for _, tc := range cases {
		raw, err := json.Marshal(tc.val)
		if err != nil {
			t.Fatalf("%s marshal: %v", tc.name, err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s unmarshal: %v", tc.name, err)
		}
		got, ok := m[tc.key]
		if !ok {
			t.Fatalf("%s: key %q missing in %s", tc.name, tc.key, string(raw))
		}
		if string(got) != tc.want {
			t.Fatalf("%s: %s = %s, want %s", tc.name, tc.key, string(got), tc.want)
		}
	}
}

// TestMarketEntryI18n 验证市场条目解析（真实 API v0.25.0 结构：{"data":[...]} + i18n.zh-hans 嵌套）。
func TestMarketEntryI18n(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"apiVersion":"0.25.0","data":[
			{"id":"1","slug":"rhino_prime_set","gameRef":"Rhino Prime Set","bulkTradable":true,"maxRank":30,"ducats":45,"vaulted":false,"i18n":{"zh-hans":{"name":"犀牛P","icon":"i1","thumb":"t1"}}},
			{"id":"2","slug":"no_i18n","gameRef":"No I18n"}
		]}`)
	}))
	defer server.Close()

	api := warframe.NewMarketAPIWithBaseURL(server.Client(), server.URL)
	items, err := api.FetchItems()
	if err != nil {
		t.Fatalf("FetchItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "犀牛P" || items[0].Icon != "i1" || items[0].Thumb != "t1" || !items[0].BulkTradable || items[0].Ducats != 45 {
		t.Fatalf("i18n resolve mismatch: %+v", items[0])
	}
	if items[1].Name != "no_i18n" {
		t.Fatalf("missing i18n should fallback to slug: %+v", items[1])
	}

	// riven 端点同样走 data 数组
	rivenItems, err := api.FetchRivenWeapons()
	if err != nil {
		t.Fatalf("FetchRivenWeapons failed: %v", err)
	}
	if len(rivenItems) != 2 {
		t.Fatalf("expected 2 riven weapons, got %d", len(rivenItems))
	}
}
