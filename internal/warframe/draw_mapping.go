// 领域数据 → draw 绘图 DTO 的转换（阶段 10 指令复用）。
// 转换保持纯函数，便于 tests 包黑盒测试；指令层仅做取数与回复。
package warframe

import (
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// ArbitrationToDraw 将仲裁领域数据转为绘图 DTO，并补齐全部显示值。
// 仲裁条目只承载 arbys.txt 的两列，节点名（含星系）/ 任务类型 / 评级均在转换时查表补齐；
// draw 包不 import 本包（会循环依赖），显示值只能在转换时填好。
func ArbitrationToDraw(arb Arbitration) *draw.Arbitration {
	return &draw.Arbitration{
		Activation:  time.Unix(arb.Activation, 0),
		Expiry:      time.Unix(arb.Expiry, 0),
		Node:        TranslateNode(arb.NodeKey),
		MissionName: nodeMissionNameByKey(arb.NodeKey),
		Faction:     arb.Faction,
		Tier:        ArbitrationTier(arb.NodeKey),
	}
}

// nodeMissionNameByKey 查询节点任务类型的中文名；未收录的序号回退空串（卡片按「未知」渲染）。
func nodeMissionNameByKey(nodeKey string) string {
	if info, ok := drawplugin.MissionTypeMap[nodeMissionTypeByKey(nodeKey)]; ok {
		return info.Name
	}
	return ""
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
