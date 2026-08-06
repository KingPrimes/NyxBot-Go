package tests

import (
	"encoding/json"
	"strings"
	"testing"

	modelwarframe "nyxbot-go/internal/model/warframe"
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
