// 世界状态查询（批次 C）：三赏金集团任务。
//
// 对齐 Java SyndicateMissionsUtils.getSyndicateMissions + AbstractSyndicatePlugin：
// 在 SyndicateMissions 中按 Tag 找到对应集团（且必须有 Jobs）→ 翻译任务类型/描述 →
// 由 rewards 关联 reward_pool 表补全奖励列表。节点视图与赏金卡片视图的分支由绘制层决定。
package warframe

import (
	"errors"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// WfSyndicate 可查询赏金的集团（对应三条指令的触发词）。
type WfSyndicate string

const (
	// SyndicateOstrons 希图斯 / 地球赏金（Tag=CetusSyndicate）
	SyndicateOstrons WfSyndicate = "CetusSyndicate"
	// SyndicateEntrati 英择谛 / 火卫二赏金（Tag=EntratiSyndicate）
	SyndicateEntrati WfSyndicate = "EntratiSyndicate"
	// SyndicateSolarisUnited 索拉里斯 / 金星赏金（Tag=SolarisSyndicate）
	SyndicateSolarisUnited WfSyndicate = "SolarisSyndicate"
)

// syndicateDrawName 集团 Tag → 绘图用中文名（对齐 Java SyndicateEnum.name）。
var syndicateDrawName = map[WfSyndicate]string{
	SyndicateOstrons:       "Ostron",
	SyndicateEntrati:       "英择谛",
	SyndicateSolarisUnited: "索拉里斯联盟",
}

// wsSyndicateEnvelope 三赏金所需字段的 WorldState 视图。
type wsSyndicateEnvelope struct {
	SyndicateMissions []wsSyndicateMissionFull `json:"SyndicateMissions"`
}

// wsSyndicateMissionFull 对齐 Java model.worldstate.SyndicateMission（含 Jobs）。
type wsSyndicateMissionFull struct {
	Tag   string           `json:"Tag"`
	Nodes []string         `json:"Nodes"`
	Jobs  []wsSyndicateJob `json:"Jobs"`
}

// wsSyndicateJob 对齐 Java model.worldstate.Job 的 JSON 字段。
type wsSyndicateJob struct {
	Type        string `json:"jobType"`
	Desc        string `json:"desc"`
	LocationTag string `json:"locationTag"`
	Rewards     string `json:"rewards"`
	MasteryReq  int    `json:"masteryReq"`
	MinLevel    int    `json:"minEnemyLevel"`
	MaxLevel    int    `json:"maxEnemyLevel"`
	XpAmounts   []int  `json:"xpAmounts"`
	Endless     bool   `json:"endless"`
	IsVault     bool   `json:"isVault"`
}

// GetSyndicate 查询指定集团的赏金任务（对齐 Java getSyndicateMissions）。
// 未找到该集团或该集团没有 Jobs 时返回空结果（不报错，由指令层提示）。
func GetSyndicate(syndicate WfSyndicate) (*draw.SyndicateMission, error) {
	env, err := parseWorldStateEnvelope[wsSyndicateEnvelope]("syndicate")
	if err != nil {
		return nil, err
	}
	for i := range env.SyndicateMissions {
		mission := &env.SyndicateMissions[i]
		if WfSyndicate(mission.Tag) != syndicate {
			continue
		}
		if len(mission.Jobs) == 0 {
			// 对齐 Java：filter(jobs != null && !isEmpty) 后再 findFirst
			continue
		}
		return buildSyndicateMission(mission, syndicate), nil
	}
	return &draw.SyndicateMission{}, nil
}

// buildSyndicateMission 组装集团任务绘图输入。
func buildSyndicateMission(mission *wsSyndicateMissionFull, syndicate WfSyndicate) *draw.SyndicateMission {
	dto := &draw.SyndicateMission{
		Tag:   &draw.SyndicateTag{Name: syndicateTagName(syndicate)},
		Nodes: mission.Nodes,
	}
	for i := range mission.Jobs {
		dto.Jobs = append(dto.Jobs, translateSyndicateJob(&mission.Jobs[i]))
	}
	return dto
}

// syndicateTagName 集团 Tag → 中文展示名（未命中回退原文）。
func syndicateTagName(syndicate WfSyndicate) string {
	if name, ok := syndicateDrawName[syndicate]; ok {
		return name
	}
	return string(syndicate)
}

// translateSyndicateJob 翻译单条赏金（对齐 Java SyndicateMissionsUtils：
// 按 jobType 或 locationTag 查 state_translation 取名称与描述，
// 再由 rewards 关联 reward_pool 表补全奖励）。
func translateSyndicateJob(job *wsSyndicateJob) *draw.SyndicateJob {
	dto := &draw.SyndicateJob{
		Type:       job.Type,
		IsVault:    job.IsVault,
		Endless:    job.Endless,
		MinLevel:   job.MinLevel,
		MaxLevel:   job.MaxLevel,
		MasteryReq: job.MasteryReq,
		Desc:       job.Desc,
		XpAmounts:  job.XpAmounts,
	}

	// 类型/描述优先按 jobType 翻译，缺失时回退 locationTag（对齐 Java 的三元选择）
	translationKey := job.Type
	if translationKey == "" {
		translationKey = job.LocationTag
	}
	if name, desc := syndicateTranslation(translationKey); name != "" {
		dto.Type = name
		if desc != "" {
			dto.Desc = desc
		}
	}

	dto.Rewards = syndicateJobRewards(job.Rewards)
	return dto
}

// syndicateTranslation 按 uniqueName 查 state_translation 的名称与描述（未命中返回空串）。
func syndicateTranslation(uniqueName string) (string, string) {
	if uniqueName == "" || database.DB == nil {
		return "", ""
	}
	var st modelwarframe.StateTranslation
	err := database.DB.Where("unique_name = ?", getLastThreeSegments(uniqueName)).First(&st).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.syndicate", "query state_translation %q failed: %v", uniqueName, err)
		}
		return "", ""
	}
	return st.Name, st.Description
}

// syndicateJobRewards 由 rewards 键关联 reward_pool 表并转换为绘图奖励列表。
func syndicateJobRewards(rewardsKey string) []*draw.SyndicateReward {
	if rewardsKey == "" || database.DB == nil {
		return nil
	}
	var pool modelwarframe.RewardPool
	err := database.DB.Preload("Rewards").Where("unique_name = ?", rewardsKey).First(&pool).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.syndicate", "query reward pool %q failed: %v", rewardsKey, err)
		}
		return nil
	}
	list := make([]*draw.SyndicateReward, 0, len(pool.Rewards))
	for i := range pool.Rewards {
		reward := &pool.Rewards[i]
		list = append(list, &draw.SyndicateReward{
			Rarity:    rarityFromOrdinal(reward.Rarity),
			Item:      reward.Item,
			ItemCount: reward.ItemCount,
		})
	}
	return list
}
