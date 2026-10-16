// 仲裁节点分级的判定与查询：本包是唯一实现，draw 层只消费评级、不重复判定
// （对齐 plans/arbitration-tier-plan.md §6.3）。
package warframe

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// worthTierThreshold 值得参与的评级阈值：评级不低于该值即值得参与（S/A/B 值得，C/D/F 与无分级不值得）。
const worthTierThreshold = "B"

// tierRanks 评级 → 序数（S 最高，F 最低）；未知评级按 F 处理。
var tierRanks = map[string]int{
	"S": 5,
	"A": 4,
	"B": 3,
	"C": 2,
	"D": 1,
	"F": 0,
}

// TierRank 返回评级序数：S=5、A=4、B=3、C=2、D=1，F/空/未知=0。
func TierRank(tier string) int {
	return tierRanks[normalizeTier(tier)]
}

// IsWorthTier 判断评级是否达到值得参与阈值。
func IsWorthTier(tier string) bool {
	return TierRank(tier) >= TierRank(worthTierThreshold)
}

// ArbitrationTier 查询节点评级；节点无分级（或未收录、数据库不可用）时返回 "F"。
func ArbitrationTier(nodeKey string) string {
	if nodeKey == "" || database.DB == nil {
		return "F"
	}
	var tier modelwarframe.ArbitrationTier
	if err := database.DB.Where(&modelwarframe.ArbitrationTier{Key: nodeKey}).First(&tier).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.arbitration", "query arbitration tier %q failed: %v", nodeKey, err)
		}
		return "F"
	}
	return normalizeTier(tier.Tier)
}

// normalizeTier 规整评级文本（去空白、转大写）；未知值统一回退 "F"。
func normalizeTier(tier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tier))
	if _, ok := tierRanks[normalized]; !ok {
		return "F"
	}
	return normalized
}
