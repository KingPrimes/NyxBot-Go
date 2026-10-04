// 本地数据管理 save/edit/remove 接口实现，对应 Java 各 controller 的 CRUD
// 语义对齐：edit 不存在返回 success(data=null) 而非 404；remove 总是成功
// AliasController.edit 返回 key 为 "alias"，其余为 "data"（对齐 Java 响应结构）
package warframe

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/response"
)

// editData 通用 edit 响应结构（对齐 Java Map.of("data", entity)）。
type editData struct {
	Data any `json:"data"`
}

// editAliasData AliasController 特例：返回 key 为 "alias"。
type editAliasData struct {
	Alias any `json:"alias"`
}

// aliasEnPattern 对齐 Java isValidEnglish 正则。
var aliasEnPattern = regexp.MustCompile(`^([a-zA-Z]+)([ _&])?([0-9]+)?([a-zA-Z]+)?$`)

// aliasCnPattern 对齐 Java isValidChinese 正则。
var aliasCnPattern = regexp.MustCompile(`^[\p{Han}]+([ ·_&])?[\p{Han}]+$`)

// AliasSave 处理 POST /data/warframe/alias/save，含非空/英文/中文/查重校验。
func (h *DataHandler) AliasSave(c *gin.Context) {
	var req warframe.Alias
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if req.Cn == "" || req.En == "" {
		response.Fail(c, http.StatusInternalServerError, "别名不能为空")
		return
	}
	if !aliasEnPattern.MatchString(req.En) {
		response.Fail(c, http.StatusInternalServerError, "英文名格式不合法")
		return
	}
	if !aliasCnPattern.MatchString(req.Cn) {
		response.Fail(c, http.StatusInternalServerError, "中文名格式不合法")
		return
	}
	var count int64
	h.db.Model(&warframe.Alias{}).Where("cn = ? AND en = ?", req.Cn, req.En).Count(&count)
	if count > 0 {
		response.Fail(c, http.StatusInternalServerError, "别名已存在")
		return
	}
	if err := h.db.Save(&req).Error; err != nil {
		response.Error(c, "保存别名失败")
		return
	}
	response.SuccessMsg(c, "保存成功", nil)
}

// AliasEdit 处理 GET /data/warframe/alias/edit/:id，不存在返回 data=null。
func (h *DataHandler) AliasEdit(c *gin.Context) {
	var record warframe.Alias
	if err := h.db.First(&record, paramInt64(c, "id")).Error; err != nil {
		response.Success(c, editAliasData{Alias: nil})
		return
	}
	response.Success(c, editAliasData{Alias: record})
}

// AliasRemove 处理 DELETE /data/warframe/alias/remove/:id。
func (h *DataHandler) AliasRemove(c *gin.Context) {
	_ = h.db.Delete(&warframe.Alias{}, paramInt64(c, "id")).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// NodesSave 处理 POST /data/warframe/nodes/save（无校验，直接 save）。
func (h *DataHandler) NodesSave(c *gin.Context) {
	var req warframe.Nodes
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if err := h.db.Save(&req).Error; err != nil {
		response.Error(c, "保存节点失败")
		return
	}
	response.SuccessMsg(c, "保存成功", nil)
}

// NodesEdit 处理 GET /data/warframe/nodes/edit/:uniqueName。
func (h *DataHandler) NodesEdit(c *gin.Context) {
	var record warframe.Nodes
	if err := h.db.First(&record, "unique_name = ?", c.Param("uniqueName")).Error; err != nil {
		response.Success(c, editData{Data: nil})
		return
	}
	response.Success(c, editData{Data: record})
}

// NodesRemove 处理 DELETE /data/warframe/nodes/remove/:uniqueName。
func (h *DataHandler) NodesRemove(c *gin.Context) {
	_ = h.db.Delete(&warframe.Nodes{}, "unique_name = ?", c.Param("uniqueName")).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// RewardPoolSave 处理 POST /data/warframe/reward-pool/save（无校验，级联保存奖励）。
func (h *DataHandler) RewardPoolSave(c *gin.Context) {
	var req warframe.RewardPool
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	err := h.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(&req).Error
	if err != nil {
		response.Error(c, "保存奖励池失败")
		return
	}
	response.SuccessMsg(c, "保存成功", nil)
}

// RewardPoolEdit 处理 GET /data/warframe/reward-pool/edit/:uniqueName（含奖励级联）。
func (h *DataHandler) RewardPoolEdit(c *gin.Context) {
	var record warframe.RewardPool
	if err := h.db.Preload("Rewards").First(&record, "unique_name = ?", c.Param("uniqueName")).Error; err != nil {
		response.Success(c, editData{Data: nil})
		return
	}
	response.Success(c, editData{Data: record})
}

// RewardPoolRemove 处理 DELETE /data/warframe/reward-pool/remove/:uniqueName。
func (h *DataHandler) RewardPoolRemove(c *gin.Context) {
	_ = h.db.Delete(&warframe.RewardPool{}, "unique_name = ?", c.Param("uniqueName")).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// RivenTionSave 处理 POST /data/warframe/riven-tion/save（无校验，直接 save）。
func (h *DataHandler) RivenTionSave(c *gin.Context) {
	var req warframe.RivenTion
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if err := h.db.Save(&req).Error; err != nil {
		response.Error(c, "保存紫卡词条失败")
		return
	}
	response.SuccessMsg(c, "保存成功", nil)
}

// RivenTionEdit 处理 GET /data/warframe/riven-tion/edit/:id。
func (h *DataHandler) RivenTionEdit(c *gin.Context) {
	var record warframe.RivenTion
	if err := h.db.First(&record, paramInt64(c, "id")).Error; err != nil {
		response.Success(c, editData{Data: nil})
		return
	}
	response.Success(c, editData{Data: record})
}

// RivenTionRemove 处理 DELETE /data/warframe/riven-tion/remove/:id。
func (h *DataHandler) RivenTionRemove(c *gin.Context) {
	_ = h.db.Delete(&warframe.RivenTion{}, paramInt64(c, "id")).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// RivenTionAliasSave 处理 POST /data/warframe/riven-tion-alias/save。
func (h *DataHandler) RivenTionAliasSave(c *gin.Context) {
	var req warframe.RivenTionAlias
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if err := h.db.Save(&req).Error; err != nil {
		response.Error(c, "保存紫卡词条别名失败")
		return
	}
	response.SuccessMsg(c, "保存成功", nil)
}

// RivenTionAliasEdit 处理 GET /data/warframe/riven-tion-alias/edit/:id。
func (h *DataHandler) RivenTionAliasEdit(c *gin.Context) {
	var record warframe.RivenTionAlias
	if err := h.db.First(&record, paramInt64(c, "id")).Error; err != nil {
		response.Success(c, editData{Data: nil})
		return
	}
	response.Success(c, editData{Data: record})
}

// RivenTionAliasRemove 处理 DELETE /data/warframe/riven-tion-alias/remove/:id。
func (h *DataHandler) RivenTionAliasRemove(c *gin.Context) {
	_ = h.db.Delete(&warframe.RivenTionAlias{}, paramInt64(c, "id")).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// StateTranslationSave 处理 POST /data/warframe/state-translation/save，校验 uniqueName/name 非空。
func (h *DataHandler) StateTranslationSave(c *gin.Context) {
	var req warframe.StateTranslation
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if req.UniqueName == "" || req.Name == "" {
		response.Fail(c, http.StatusInternalServerError, "uniqueName 与 name 不能为空")
		return
	}
	if err := h.db.Save(&req).Error; err != nil {
		response.Error(c, "保存状态翻译失败")
		return
	}
	response.SuccessMsg(c, "保存成功", nil)
}

// StateTranslationEdit 处理 GET /data/warframe/state-translation/edit/:uniqueName。
func (h *DataHandler) StateTranslationEdit(c *gin.Context) {
	var record warframe.StateTranslation
	if err := h.db.First(&record, "unique_name = ?", c.Param("uniqueName")).Error; err != nil {
		response.Success(c, editData{Data: nil})
		return
	}
	response.Success(c, editData{Data: record})
}

// StateTranslationRemove 处理 DELETE /data/warframe/state-translation/remove/:uniqueName。
func (h *DataHandler) StateTranslationRemove(c *gin.Context) {
	_ = h.db.Delete(&warframe.StateTranslation{}, "unique_name = ?", c.Param("uniqueName")).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// stateTypeOption 状态类型下拉选项（对齐 Java /types 的 {value, label}）。
type stateTypeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// StateTranslationTypes 处理 GET /data/warframe/state-translation/types，
// 返回 StateTypeEnum 全部值 {value=枚举名, label=中文名}（对齐 Java StateTranslationController.types()）。
// 数据源为 modelwarframe.StateTypes 单一事实来源，不再维护平行映射。
func (h *DataHandler) StateTranslationTypes(c *gin.Context) {
	options := make([]stateTypeOption, 0, len(warframe.StateTypes))
	for _, definition := range warframe.StateTypes {
		options = append(options, stateTypeOption{Value: definition.Name, Label: definition.Label})
	}
	response.Success(c, options)
}
