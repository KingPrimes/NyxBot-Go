// 领域数据 → draw 绘图 DTO 的转换（阶段 10 指令复用）。
// 转换保持纯函数，便于 tests 包黑盒测试；指令层仅做取数与回复。
package warframe

import (
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// ArbitrationToDraw 将仲裁领域数据转为绘图 DTO。
// 仲裁 API（node/planet/enemy/type）本身返回中文，无需翻译。
func ArbitrationToDraw(arb Arbitration) *draw.Arbitration {
	return &draw.Arbitration{
		Activation: parseRFC3339(arb.Activation),
		Expiry:     parseRFC3339(arb.Expiry),
		ID:         arb.ID,
		Node:       arb.Node,
		Planet:     arb.Planet,
		Enemy:      arb.Enemy,
		Type:       arb.Type,
	}
}

// RelicsToDraw 将遗物模型转为绘图 DTO，并把稀有度序数映射为枚举。
func RelicsToDraw(relic *modelwarframe.Relics) *draw.Relics {
	if relic == nil {
		return nil
	}
	dto := &draw.Relics{Name: relic.Name}
	dto.Rewards = make([]*draw.RelicReward, 0, len(relic.RelicRewards))
	for _, reward := range relic.RelicRewards {
		itemCount := reward.ItemCount
		dto.Rewards = append(dto.Rewards, &draw.RelicReward{
			Name:      reward.RewardName,
			Rarity:    rarityFromOrdinal(reward.Rarity),
			ItemCount: &itemCount,
		})
	}
	return dto
}

// rarityFromOrdinal 将稀有度序数映射为 drawplugin.Rarity 枚举。
// 序数 0..3 依次对应 COMMON/UNCOMMON/RARE/LEGENDARY（对齐 Java 稀有度序列号）。
func rarityFromOrdinal(ordinal int) drawplugin.Rarity {
	switch ordinal {
	case 1:
		return drawplugin.RarityUncommon
	case 2:
		return drawplugin.RarityRare
	case 3:
		return drawplugin.RarityLegendary
	default:
		return drawplugin.RarityCommon
	}
}

// parseRFC3339 解析 RFC3339 时间；失败返回零值。
func parseRFC3339(text string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, text)
	return parsed
}
