// 数据更新任务与 SSE 进度推送，对应 Java NyxBot 的 DataRefreshEvent + LogSseController
// POST /data/warframe/{module}/update 立即返回"请求任务执行中"，异步执行导入任务，
// 完成后经 /sse/data-refresh 推送 STARTED/COMPLETED/FAILED 事件
package warframe

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/response"
)

// taskStatus 数据刷新任务状态（对齐 Java DataRefreshEvent 的枚举）。
type taskStatus string

const (
	taskStarted   taskStatus = "STARTED"
	taskCompleted taskStatus = "COMPLETED"
	taskFailed    taskStatus = "FAILED"
)

// refreshEvent SSE data-refresh 事件体（对齐 Java 推送的 JSON 结构）。
type refreshEvent struct {
	Type     string     `json:"type"`     // 恒为 "dataRefresh"
	TaskName string     `json:"taskName"` // 中文任务名（i18n 解析后）
	Status   taskStatus `json:"status"`   // STARTED / COMPLETED / FAILED
	Message  string     `json:"message"`  // 提示信息
}

// DataUpdater 管理异步数据刷新任务与 SSE 订阅者广播。
type DataUpdater struct {
	importer *DataImporter

	mu      sync.Mutex
	subs    map[chan refreshEvent]struct{} // SSE 订阅者
	running map[string]bool                // 任务名 -> 是否执行中（防重入）
}

// NewDataUpdater 创建数据更新器。
func NewDataUpdater(importer *DataImporter) *DataUpdater {
	return &DataUpdater{
		importer: importer,
		subs:     make(map[chan refreshEvent]struct{}),
		running:  make(map[string]bool),
	}
}

// runTask 执行一个异步刷新任务（对齐 Java DataRefreshEvent.runAsync）：
// 先发 STARTED，执行任务，成功发 COMPLETED，失败发 FAILED。任务并发由 running 去重。
func (updater *DataUpdater) runTask(taskName string, task func() error) {
	updater.mu.Lock()
	if updater.running[taskName] {
		updater.mu.Unlock()
		logging.DebugPack("warframe.update", "task %s already running, skip", taskName)
		return
	}
	updater.running[taskName] = true
	updater.mu.Unlock()
	defer func() {
		updater.mu.Lock()
		delete(updater.running, taskName)
		updater.mu.Unlock()
	}()

	updater.broadcast(refreshEvent{Type: "dataRefresh", TaskName: taskName, Status: taskStarted, Message: fmt.Sprintf("正在%s...", taskName)})
	logging.InfoPack("warframe.update", "task %s started", taskName)

	// 任务开始前清除市场缓存，确保任务内重新拉取最新数据（对齐 Java 更新后重建缓存）。
	updater.importer.market.Invalidate()

	if err := task(); err != nil {
		message := fmt.Sprintf("%s失败: %v", taskName, err)
		updater.broadcast(refreshEvent{Type: "dataRefresh", TaskName: taskName, Status: taskFailed, Message: message})
		logging.ErrorPack("warframe.update", "task %s failed: %v", taskName, err)
		return
	}
	updater.broadcast(refreshEvent{Type: "dataRefresh", TaskName: taskName, Status: taskCompleted, Message: fmt.Sprintf("%s完成", taskName)})
	logging.InfoPack("warframe.update", "task %s completed", taskName)
}

// trigger 由 update 接口调用：立即返回，异步执行任务。
func (updater *DataUpdater) trigger(taskName string, task func() error) {
	go updater.runTask(taskName, task)
}

// broadcast 向全部 SSE 订阅者推送事件（非阻塞，订阅者掉线自动移除）。
func (updater *DataUpdater) broadcast(event refreshEvent) {
	updater.mu.Lock()
	defer updater.mu.Unlock()
	for subscriber := range updater.subs {
		select {
		case subscriber <- event:
		default:
		}
	}
}

// Subscribe 注册 SSE 订阅者，返回事件通道；退订时调用 unsubscribe。
func (updater *DataUpdater) Subscribe() (chan refreshEvent, func()) {
	channel := make(chan refreshEvent, 16)
	updater.mu.Lock()
	updater.subs[channel] = struct{}{}
	updater.mu.Unlock()
	return channel, func() {
		updater.mu.Lock()
		delete(updater.subs, channel)
		updater.mu.Unlock()
	}
}

// HandleDataRefreshSSE 处理 GET /sse/data-refresh：推送数据刷新进度事件流。
// 对齐 Java LogSseController：先发 connected 事件带 sessionId，之后 data-refresh 事件。
func (updater *DataUpdater) HandleDataRefreshSSE(c *gin.Context) {
	channel, unsubscribe := updater.Subscribe()
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 首帧 connected 事件（对齐 Java）
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"sessionId\":\"%s\"}\n\n", sessionID)
	c.Writer.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event := <-channel:
			fmt.Fprintf(c.Writer, "event: data-refresh\ndata: %s\n\n", eventJSON(event))
			c.Writer.Flush()
		case <-heartbeat.C:
			fmt.Fprint(c.Writer, ": keep-alive\n\n")
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// eventJSON 序列化 refreshEvent（供 SSE 输出）。
func eventJSON(event refreshEvent) string {
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Sprintf(`{"type":"dataRefresh","taskName":%q,"status":%q,"message":%q}`, event.TaskName, event.Status, event.Message)
	}
	return string(encoded)
}

// UpdateAlias 处理 POST /data/warframe/alias/update。
func (h *DataHandler) UpdateAlias(c *gin.Context) {
	h.updateRespond(c, "别名数据", h.importer.ImportAlias)
}

// UpdateEphemeras 处理 POST /data/warframe/ephemeras/update。
func (h *DataHandler) UpdateEphemeras(c *gin.Context) {
	h.updateRespond(c, "幻纹数据", h.importer.UpdateEphemeras)
}

// UpdateLichSister 处理 POST /data/warframe/lich-sister/update。
func (h *DataHandler) UpdateLichSister(c *gin.Context) {
	h.updateRespond(c, "赤毒信条武器数据", h.importer.UpdateLichSister)
}

// UpdateMarket 处理 POST /data/warframe/market/update。
func (h *DataHandler) UpdateMarket(c *gin.Context) {
	h.updateRespond(c, "市场物品数据", h.importer.UpdateOrdersItems)
}

// UpdateMarketRiven 处理 POST /data/warframe/market/riven/update。
func (h *DataHandler) UpdateMarketRiven(c *gin.Context) {
	h.updateRespond(c, "紫卡武器数据", h.importer.UpdateRivenItems)
}

// UpdateNightWave 处理 POST /data/warframe/night-wave/update。
func (h *DataHandler) UpdateNightWave(c *gin.Context) {
	h.updateRespond(c, "电波数据", h.importer.ImportNightWave)
}

// UpdateNodes 处理 POST /data/warframe/nodes/update。
func (h *DataHandler) UpdateNodes(c *gin.Context) {
	h.updateRespond(c, "节点数据", h.importer.ImportNodes)
}

// UpdateRewardPool 处理 POST /data/warframe/reward-pool/update。
func (h *DataHandler) UpdateRewardPool(c *gin.Context) {
	h.updateRespond(c, "奖励池数据", h.importer.ImportRewardPool)
}

// UpdateRivenAnalyse 处理 POST /data/warframe/riven-analyse/update。
func (h *DataHandler) UpdateRivenAnalyse(c *gin.Context) {
	h.updateRespond(c, "紫卡分析趋势数据", h.importer.ImportRivenAnalyseTrend)
}

// UpdateRivenTion 处理 POST /data/warframe/riven-tion/update。
func (h *DataHandler) UpdateRivenTion(c *gin.Context) {
	h.updateRespond(c, "紫卡词条数据", h.importer.ImportRivenTion)
}

// UpdateRivenTionAlias 处理 POST /data/warframe/riven-tion-alias/update。
func (h *DataHandler) UpdateRivenTionAlias(c *gin.Context) {
	h.updateRespond(c, "紫卡词条别名数据", h.importer.ImportRivenTionAlias)
}

// UpdateStateTranslation 处理 POST /data/warframe/state-translation/update。
func (h *DataHandler) UpdateStateTranslation(c *gin.Context) {
	h.updateRespond(c, "状态翻译数据", h.importer.ImportStateTranslation)
}

// UpdateWarframes 处理 POST /data/warframe/warframes/update。
func (h *DataHandler) UpdateWarframes(c *gin.Context) {
	h.updateRespond(c, "战甲数据", h.importer.ImportWarframes)
}

// UpdateWeapons 处理 POST /data/warframe/weapons/update。
func (h *DataHandler) UpdateWeapons(c *gin.Context) {
	h.updateRespond(c, "武器数据", h.importer.ImportWeapons)
}

// UpdateRelics 处理 POST /data/warframe/relics/update。
func (h *DataHandler) UpdateRelics(c *gin.Context) {
	h.updateRespond(c, "遗物数据", h.importer.ImportRelics)
}

// updateRespond 统一 update 接口行为：立即返回"请求任务执行中"（对齐 Java RequestTaskRun），
// 实际任务异步执行，经 SSE data-refresh 推送进度。
func (h *DataHandler) updateRespond(c *gin.Context, taskName string, task func(context.Context) error) {
	if h.updater == nil {
		response.Error(c, "数据更新器未初始化")
		return
	}
	h.updater.trigger(taskName, func() error {
		return task(context.Background())
	})
	response.SuccessMsg(c, "请求任务执行中", nil)
}

// RunUpdateTasks 供 main 调用，注册上下文感知的导出更新任务（占位，后续扩展）。
func (updater *DataUpdater) RunUpdateTasks(_ context.Context) {}
