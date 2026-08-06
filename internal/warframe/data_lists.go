// 本地数据管理 list 接口实现，对应 Java 各 controller 的 findAllPageable
// 分页统一：body {current, size, 过滤字段} → {total, size, current, records}
package warframe

import (
	"github.com/gin-gonic/gin"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// AliasList 处理 POST /data/warframe/alias/list，cn 模糊过滤（对齐 findByLikeCn）。
func (h *DataHandler) AliasList(c *gin.Context) {
	var req struct {
		listRequest
		Cn string `json:"cn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.bindList(c)
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.Alias{})
	if req.Cn != "" {
		query = query.Where("LOWER(cn) LIKE ?", likePattern(req.Cn))
	}
	var total int64
	query.Count(&total)
	var records []modelwarframe.Alias
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// EphemerasList 处理 POST /data/warframe/ephemeras/list，name 模糊过滤。
func (h *DataHandler) EphemerasList(c *gin.Context) {
	var req struct {
		listRequest
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.bindList(c)
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.Ephemera{})
	if req.Name != "" {
		query = query.Where("LOWER(name) LIKE ?", likePattern(req.Name))
	}
	var total int64
	query.Count(&total)
	var records []modelwarframe.Ephemera
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// LichSisterList 处理 POST /data/warframe/lich-sister/list，无过滤全量分页。
func (h *DataHandler) LichSisterList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.LichSisterWeapon{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.LichSisterWeapon
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// MarketList 处理 POST /data/warframe/market/list，name 模糊过滤。
func (h *DataHandler) MarketList(c *gin.Context) {
	var req struct {
		listRequest
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.bindList(c)
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.OrdersItem{})
	if req.Name != "" {
		query = query.Where("LOWER(name) LIKE ?", likePattern(req.Name))
	}
	var total int64
	query.Count(&total)
	var records []modelwarframe.OrdersItem
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// MarketRivenList 处理 POST /data/warframe/market/riven/list，name 模糊 + rivenType 等值。
func (h *DataHandler) MarketRivenList(c *gin.Context) {
	var req struct {
		listRequest
		Name      string `json:"name"`
		RivenType string `json:"rivenType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.bindList(c)
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.RivenItem{})
	if req.Name != "" {
		query = query.Where("LOWER(name) LIKE ?", likePattern(req.Name))
	}
	if req.RivenType != "" {
		query = query.Where("riven_type = ?", req.RivenType)
	}
	var total int64
	query.Count(&total)
	var records []modelwarframe.RivenItem
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// NightWaveList 处理 POST /data/warframe/night-wave/list，全量分页。
func (h *DataHandler) NightWaveList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.NightWave{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.NightWave
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// NodesList 处理 POST /data/warframe/nodes/list，全量分页。
func (h *DataHandler) NodesList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.Nodes{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.Nodes
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// RewardPoolList 处理 POST /data/warframe/reward-pool/list，全量分页（含奖励级联）。
func (h *DataHandler) RewardPoolList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.RewardPool{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.RewardPool
	query.Preload("Rewards").Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// RivenAnalyseList 处理 POST /data/warframe/riven-analyse/list，全量分页。
func (h *DataHandler) RivenAnalyseList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.RivenAnalyseTrend{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.RivenAnalyseTrend
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// RivenTionList 处理 POST /data/warframe/riven-tion/list，全量分页。
func (h *DataHandler) RivenTionList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.RivenTion{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.RivenTion
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// RivenTionAliasList 处理 POST /data/warframe/riven-tion-alias/list，全量分页。
func (h *DataHandler) RivenTionAliasList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.RivenTionAlias{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.RivenTionAlias
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// StateTranslationList 处理 POST /data/warframe/state-translation/list，全量分页。
func (h *DataHandler) StateTranslationList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.StateTranslation{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.StateTranslation
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// WarframesList 处理 POST /data/warframe/warframes/list，全量分页（含技能级联）。
func (h *DataHandler) WarframesList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.Warframes{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.Warframes
	query.Preload("Abilities").Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// WeaponsList 处理 POST /data/warframe/weapons/list，全量分页。
func (h *DataHandler) WeaponsList(c *gin.Context) {
	req, ok := h.bindList(c)
	if !ok {
		return
	}
	offset, limit := h.normalizePage(req)
	query := h.db.Model(&modelwarframe.Weapons{})
	var total int64
	query.Count(&total)
	var records []modelwarframe.Weapons
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, req, records)
}

// RelicsList 处理 POST /data/warframe/relics/list，name 等值过滤（对齐 findAllPageable 等值语义）。
func (h *DataHandler) RelicsList(c *gin.Context) {
	var req struct {
		listRequest
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.bindList(c)
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.Relics{})
	if req.Name != "" {
		query = query.Where("LOWER(name) = LOWER(?)", req.Name)
	}
	var total int64
	query.Count(&total)
	var records []modelwarframe.Relics
	query.Preload("RelicRewards").Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}
