// 订阅管理接口，对应 Java NyxBot 的 MissionSubscribeController
// 提供订阅类型枚举、三级分页查询（订阅组/用户/检查类型）与删除
// 供阶段 10 订阅指令与阶段 11 仲裁通知复用
package warframe

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nyxbot-go/internal/enum/nyxbot"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/response"
)

// enumOption 枚举下拉选项 {label, value}（对齐 WebUI 的 Option 结构）。
type enumOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SubscribeSubEnums 处理 GET /data/warframe/subscribe/sub，返回 SubscribeType 枚举。
func (h *DataHandler) SubscribeSubEnums(c *gin.Context) {
	response.Success(c, subscribeEnums())
}

// SubscribeTypeEnums 处理 GET /data/warframe/subscribe/type，返回 MissionType 枚举。
func (h *DataHandler) SubscribeTypeEnums(c *gin.Context) {
	response.Success(c, missionTypeEnums())
}

// SubscribeRewardEnums 处理 GET /data/warframe/subscribe/reward，返回 InvasionReward 枚举。
func (h *DataHandler) SubscribeRewardEnums(c *gin.Context) {
	response.Success(c, invasionRewardEnums())
}

// subscribeEnums SubscribeType 全部 14 项（对齐 Java SubscribeType 枚举顺序）。
func subscribeEnums() []enumOption {
	values := []struct {
		value nyxbot.SubscribeType
		label string
	}{
		{nyxbot.SubAlerts, "警报"}, {nyxbot.SubArbitration, "仲裁"},
		{nyxbot.SubCetusCycle, "夜灵平野"}, {nyxbot.SubDailyDeals, "每日特惠"},
		{nyxbot.SubEvents, "活动"}, {nyxbot.SubInvasions, "入侵"},
		{nyxbot.SubSteelPath, "钢铁兑换"}, {nyxbot.SubVoid, "奸商"},
		{nyxbot.SubFissures, "裂隙"}, {nyxbot.SubNews, "新闻"},
		{nyxbot.SubNightwave, "电波"}, {nyxbot.SubSortie, "突击"},
		{nyxbot.SubArchonHunt, "执政官突击"}, {nyxbot.SubDuviriCycle, "双衍王境"},
	}
	options := make([]enumOption, 0, len(values))
	for _, item := range values {
		options = append(options, enumOption{Label: item.label, Value: string(item.value)})
	}
	return options
}

// missionTypeEnums MissionType 全部 27 项（对齐 Java 订阅用 MissionType 枚举）。
func missionTypeEnums() []enumOption {
	values := []struct {
		value nyxbot.MissionType
		label string
	}{
		{nyxbot.MTExtermination, "歼灭"}, {nyxbot.MTSurvival, "生存"},
		{nyxbot.MTRescue, "救援"}, {nyxbot.MTSabotage, "破坏"},
		{nyxbot.MTCapture, "捕获"}, {nyxbot.MTIntel, "间谍"},
		{nyxbot.MTDefense, "防御"}, {nyxbot.MTMobileDefense, "移动防御"},
		{nyxbot.MTTerritory, "拦截"}, {nyxbot.MTHive, "清巢"},
		{nyxbot.MTRetrieval, "劫持"}, {nyxbot.MTExcavate, "挖掘"},
		{nyxbot.MTSalvage, "资源回收"}, {nyxbot.MTPursuit, "追击"},
		{nyxbot.MTAssault, "强袭"}, {nyxbot.MTEvacuation, "叛逃"},
		{nyxbot.MTDisruption, "中断"}, {nyxbot.MTVoidFlood, "虚空洪流"},
		{nyxbot.MTVoidCascade, "虚空覆涌"}, {nyxbot.MTVoidArmageddon, "虚空决战"},
		{nyxbot.MTAlchemy, "元素转换"}, {nyxbot.MTCambire, "异化区"},
		{nyxbot.MTSkirmish, "前哨战"}, {nyxbot.MTVolatile, "爆发"},
		{nyxbot.MTOrpheus, "奧菲斯"}, {nyxbot.MTAscension, "扬升"},
		{nyxbot.MTCorruption, "虚空腐蚀"},
	}
	options := make([]enumOption, 0, len(values))
	for _, item := range values {
		options = append(options, enumOption{Label: item.label, Value: string(item.value)})
	}
	return options
}

// invasionRewardEnums InvasionReward 全部 8 项（对齐 Java InvasionReward 枚举）。
func invasionRewardEnums() []enumOption {
	values := []struct {
		value nyxbot.InvasionReward
		label string
	}{
		{nyxbot.InvRewardNone, "无"}, {nyxbot.InvRewardDetoniteInjector, "突变原聚合物"},
		{nyxbot.InvRewardFieldron, "力场装置样本"}, {nyxbot.InvRewardMutagenMass, "诱变剂物质"},
		{nyxbot.InvRewardOrokinCatalyst, "Orokin 催化剂"}, {nyxbot.InvRewardOrokinReactor, "Orokin 反应堆"},
		{nyxbot.InvRewardForma, "Forma"}, {nyxbot.InvRewardExilusAdapter, "特殊功能槽连接器"},
	}
	options := make([]enumOption, 0, len(values))
	for _, item := range values {
		options = append(options, enumOption{Label: item.label, Value: string(item.value)})
	}
	return options
}

// SubscribeList 处理 POST /data/warframe/subscribe/list，按 subGroup 等值过滤分页。
func (h *DataHandler) SubscribeList(c *gin.Context) {
	var req struct {
		listRequest
		SubGroup *int64 `json:"subGroup"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.MissionSubscribe{})
	if req.SubGroup != nil {
		query = query.Where("sub_group = ?", *req.SubGroup)
	}
	var total int64
	query.Count(&total)
	var records []modelwarframe.MissionSubscribe
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// SubscribeUserList 处理 POST /data/warframe/subscribe/user/list，按订阅组 id 查用户分页。
func (h *DataHandler) SubscribeUserList(c *gin.Context) {
	var req struct {
		listRequest
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if req.ID == 0 {
		response.Fail(c, http.StatusBadRequest, "订阅组 id 不能为空")
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.MissionSubscribeUser{}).Where("sub_id = ?", req.ID)
	var total int64
	query.Count(&total)
	var records []modelwarframe.MissionSubscribeUser
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// SubscribeCheckTypeList 处理 POST /data/warframe/subscribe/type/list，按用户 id 查检查类型分页。
func (h *DataHandler) SubscribeCheckTypeList(c *gin.Context) {
	var req struct {
		listRequest
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if req.ID == 0 {
		response.Fail(c, http.StatusBadRequest, "用户 id 不能为空")
		return
	}
	offset, limit := h.normalizePage(&req.listRequest)
	query := h.db.Model(&modelwarframe.MissionSubscribeUserCheckType{}).Where("subu_id = ?", req.ID)
	var total int64
	query.Count(&total)
	var records []modelwarframe.MissionSubscribeUserCheckType
	query.Offset(offset).Limit(limit).Find(&records)
	h.pageResponse(c, total, &req.listRequest, records)
}

// SubscribeRemove 处理 DELETE /data/warframe/subscribe/:id，删除订阅组（级联用户与检查类型）。
func (h *DataHandler) SubscribeRemove(c *gin.Context) {
	id := paramInt64(c, "id")
	h.db.Where("sub_id = ?", id).Delete(&modelwarframe.MissionSubscribeUser{})
	_ = h.db.Delete(&modelwarframe.MissionSubscribe{}, id).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// SubscribeUserRemove 处理 DELETE /data/warframe/subscribe/user/:id，删除订阅用户（级联检查类型）。
func (h *DataHandler) SubscribeUserRemove(c *gin.Context) {
	id := paramInt64(c, "id")
	h.db.Where("subu_id = ?", id).Delete(&modelwarframe.MissionSubscribeUserCheckType{})
	_ = h.db.Delete(&modelwarframe.MissionSubscribeUser{}, id).Error
	response.SuccessMsg(c, "删除成功", nil)
}

// SubscribeCheckTypeRemove 处理 DELETE /data/warframe/subscribe/type/:id，删除单个检查类型。
func (h *DataHandler) SubscribeCheckTypeRemove(c *gin.Context) {
	_ = h.db.Delete(&modelwarframe.MissionSubscribeUserCheckType{}, paramInt64(c, "id")).Error
	response.SuccessMsg(c, "删除成功", nil)
}
