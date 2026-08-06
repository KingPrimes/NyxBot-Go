// 本地数据管理 HTTP 接口，对应 Java NyxBot 的 warframe controller 包
// 提供 /data/warframe/** 全部接口：分页列表、保存/编辑/删除、异步数据更新
// 前端契约：POST /{module}/list（body {current, size, 过滤字段}）→ PageData
package warframe

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/response"
)

// DataHandler Warframe 本地数据管理处理器。
type DataHandler struct {
	db       *gorm.DB
	importer *DataImporter
	updater  *DataUpdater
}

// NewDataHandler 创建数据管理处理器。
func NewDataHandler(importer *DataImporter, updater *DataUpdater) *DataHandler {
	return &DataHandler{
		db:       database.DB,
		importer: importer,
		updater:  updater,
	}
}

// listRequest 通用分页请求：current/size（1 起始）+ 模块自定义过滤字段。
type listRequest struct {
	Current int `json:"current"`
	Size    int `json:"size"`
}

// normalizePage 规范化分页参数（对齐 Java PageRequest.of(current-1, size)）。
func (h *DataHandler) normalizePage(req *listRequest) (offset, limit int) {
	if req.Current < 1 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = 15
	}
	return (req.Current - 1) * req.Size, req.Size
}

// pageResponse 组装分页响应（对齐 Java PageData{total, size, current, records}）。
func (h *DataHandler) pageResponse(c *gin.Context, total int64, req *listRequest, records any) {
	response.Page(c, total, req.Size, req.Current, records)
}

// bindList 绑定列表请求体；JSON 非法时返回 400 并终止。
func (h *DataHandler) bindList(c *gin.Context) (*listRequest, bool) {
	var req listRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return nil, false
	}
	return &req, true
}

// likePattern 构造模糊匹配参数（对齐 Java LOWER(x) like %v%）。
func likePattern(value string) string {
	return "%" + strings.ToLower(strings.TrimSpace(value)) + "%"
}

// paramInt64 解析路径参数为 int64；非法时返回 0。
func paramInt64(c *gin.Context, name string) int64 {
	value, _ := strconv.ParseInt(c.Param(name), 10, 64)
	return value
}

// Register 注册全部 /data/warframe/** 路由（由 server 路由层挂载到 RequireAuth 组，
// 传入的 routes 已带 /data 前缀）。
func (h *DataHandler) Register(routes *gin.RouterGroup) {
	data := routes.Group("/warframe")

	// alias：list / update / save / edit / remove
	alias := data.Group("/alias")
	alias.POST("/list", h.AliasList)
	alias.POST("/update", h.UpdateAlias)
	alias.POST("/save", h.AliasSave)
	alias.GET("/edit/:id", h.AliasEdit)
	alias.DELETE("/remove/:id", h.AliasRemove)

	// ephemeras：list / update
	ephemeras := data.Group("/ephemeras")
	ephemeras.POST("/list", h.EphemerasList)
	ephemeras.POST("/update", h.UpdateEphemeras)

	// lich-sister：list / update
	lichSister := data.Group("/lich-sister")
	lichSister.POST("/list", h.LichSisterList)
	lichSister.POST("/update", h.UpdateLichSister)

	// market：list / update
	market := data.Group("/market")
	market.POST("/list", h.MarketList)
	market.POST("/update", h.UpdateMarket)

	// market/riven：list / update
	marketRiven := market.Group("/riven")
	marketRiven.POST("/list", h.MarketRivenList)
	marketRiven.POST("/update", h.UpdateMarketRiven)

	// night-wave：list / update
	nightWave := data.Group("/night-wave")
	nightWave.POST("/list", h.NightWaveList)
	nightWave.POST("/update", h.UpdateNightWave)

	// nodes：list / update / save / edit / remove
	nodes := data.Group("/nodes")
	nodes.POST("/list", h.NodesList)
	nodes.POST("/update", h.UpdateNodes)
	nodes.POST("/save", h.NodesSave)
	nodes.GET("/edit/:uniqueName", h.NodesEdit)
	nodes.DELETE("/remove/:uniqueName", h.NodesRemove)

	// reward-pool：list / update / save / edit / remove
	rewardPool := data.Group("/reward-pool")
	rewardPool.POST("/list", h.RewardPoolList)
	rewardPool.POST("/update", h.UpdateRewardPool)
	rewardPool.POST("/save", h.RewardPoolSave)
	rewardPool.GET("/edit/:uniqueName", h.RewardPoolEdit)
	rewardPool.DELETE("/remove/:uniqueName", h.RewardPoolRemove)

	// riven-analyse：list / update
	rivenAnalyse := data.Group("/riven-analyse")
	rivenAnalyse.POST("/list", h.RivenAnalyseList)
	rivenAnalyse.POST("/update", h.UpdateRivenAnalyse)

	// riven-tion：list / update / save / edit / remove
	rivenTion := data.Group("/riven-tion")
	rivenTion.POST("/list", h.RivenTionList)
	rivenTion.POST("/update", h.UpdateRivenTion)
	rivenTion.POST("/save", h.RivenTionSave)
	rivenTion.GET("/edit/:id", h.RivenTionEdit)
	rivenTion.DELETE("/remove/:id", h.RivenTionRemove)

	// riven-tion-alias：list / update / save / edit / remove
	rivenTionAlias := data.Group("/riven-tion-alias")
	rivenTionAlias.POST("/list", h.RivenTionAliasList)
	rivenTionAlias.POST("/update", h.UpdateRivenTionAlias)
	rivenTionAlias.POST("/save", h.RivenTionAliasSave)
	rivenTionAlias.GET("/edit/:id", h.RivenTionAliasEdit)
	rivenTionAlias.DELETE("/remove/:id", h.RivenTionAliasRemove)

	// state-translation：list / update / save / edit / remove / types
	stateTranslation := data.Group("/state-translation")
	stateTranslation.POST("/list", h.StateTranslationList)
	stateTranslation.POST("/update", h.UpdateStateTranslation)
	stateTranslation.POST("/save", h.StateTranslationSave)
	stateTranslation.GET("/edit/:uniqueName", h.StateTranslationEdit)
	stateTranslation.DELETE("/remove/:uniqueName", h.StateTranslationRemove)
	stateTranslation.GET("/types", h.StateTranslationTypes)

	// warframes：list / update
	warframes := data.Group("/warframes")
	warframes.POST("/list", h.WarframesList)
	warframes.POST("/update", h.UpdateWarframes)

	// weapons：list / update
	weapons := data.Group("/weapons")
	weapons.POST("/list", h.WeaponsList)
	weapons.POST("/update", h.UpdateWeapons)

	// relics：list / update
	relics := data.Group("/relics")
	relics.POST("/list", h.RelicsList)
	relics.POST("/update", h.UpdateRelics)

	// subscribe：枚举 + 三级分页/删除
	subscribe := data.Group("/subscribe")
	subscribe.GET("/sub", h.SubscribeSubEnums)
	subscribe.GET("/type", h.SubscribeTypeEnums)
	subscribe.GET("/reward", h.SubscribeRewardEnums)
	subscribe.POST("/list", h.SubscribeList)
	subscribe.POST("/user/list", h.SubscribeUserList)
	subscribe.POST("/type/list", h.SubscribeCheckTypeList)
	subscribe.DELETE("/:id", h.SubscribeRemove)
	subscribe.DELETE("/user/:id", h.SubscribeUserRemove)
	subscribe.DELETE("/type/:id", h.SubscribeCheckTypeRemove)
}
