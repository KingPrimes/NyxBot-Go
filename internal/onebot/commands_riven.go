package onebot

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // 紫卡截图常见 JPEG
	_ "image/png"  // 或 PNG
	"net/http"
	"strings"
	"sync"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/ocr"
	"nyxbot-go/internal/warframe/rivenanalyse"
)

const (
	// rivenPendingTimeout 等待补发紫卡截图的超时（对齐 Java PENDING_TIMEOUT_MS）。
	rivenPendingTimeout = 60 * time.Second
	// rivenPendingPriority 补图匹配器优先级。ZeroBot 优先级数值越小越先执行（matcher.go 注释），
	// 必须排在全部指令 matcher（CodesOrder 序号 +10，最大约 50）之后，否则会拦截所有消息
	// 事件并阻断指令分发；取 100 留足余量。
	rivenPendingPriority = 100
	// rivenImageTimeout 下载单张消息图片的超时。
	rivenImageTimeout = 30 * time.Second
)

// pendingRivenRequest 等待补图的紫卡分析请求快照（对齐 Java PendingRequest）。
type pendingRivenRequest struct {
	at    time.Time
	timer *time.Timer
}

// rivenPending 按「群号:用户号」记录的等待补图请求（群聊与私聊均可，私聊群号为 0）。
var rivenPending sync.Map // map[string]*pendingRivenRequest

// rivenPendingKey 生成等待状态键（群聊 groupId:userId；私聊 0:userId）。
func rivenPendingKey(ctx *zero.Ctx) string {
	return fmt.Sprintf("%d:%d", ctx.Event.GroupID, ctx.Event.UserID)
}

// clearRivenPending 清除指定键的等待状态并停止其超时任务。
func clearRivenPending(key string) {
	if v, ok := rivenPending.LoadAndDelete(key); ok {
		if req, ok := v.(*pendingRivenRequest); ok && req.timer != nil {
			req.timer.Stop()
		}
	}
}

// wfRivenAnalyse 紫卡分析指令（对齐 Java RivenAnalysePlugin.rivenAnalyse）：
// 命令+图同消息 → 直接分析；仅命令 → 记录等待状态并提示补图（60 秒超时通知）。
func (registry *CommandRegistry) wfRivenAnalyse(ctx *zero.Ctx, _ string) error {
	key := rivenPendingKey(ctx)
	if urls := messageImageURLs(ctx); len(urls) > 0 {
		clearRivenPending(key)
		registry.replyRivenAnalyse(ctx, urls)
		return nil
	}

	// 仅命令：记录等待状态并注册超时任务（对齐 Java：60 秒后未收到图片则通知）
	req := &pendingRivenRequest{at: time.Now()}
	req.timer = time.AfterFunc(rivenPendingTimeout, func() {
		// 精准匹配：仅当仍是同一条请求时才删除并通知，避免误删用户重新发起的请求
		if v, ok := rivenPending.Load(key); ok && v == req {
			rivenPending.Delete(key)
			if err := ReplyText(ctx, "紫卡截图等待已超时（60 秒），请重新发送指令"); err != nil {
				logging.WarnPack("onebot.riven", "发送等待超时通知失败: %v", err)
			}
		}
	})
	rivenPending.Store(key, req)
	return ReplyText(ctx, "请在 60 秒内发送紫卡截图")
}

// registerRivenPendingHandler 注册「先发命令后补图片」的匹配处理器
// （对齐 Java pendingRivenHandler；手机端无法同时发送命令与图片的场景）。
func (registry *CommandRegistry) registerRivenPendingHandler() {
	registry.engine.
		On("message", zero.Type("message")).
		SetPriority(rivenPendingPriority).
		Handle(func(ctx *zero.Ctx) {
			if ctx == nil || ctx.Event == nil {
				return
			}
			key := rivenPendingKey(ctx)
			v, ok := rivenPending.Load(key)
			if !ok {
				return
			}
			req, ok := v.(*pendingRivenRequest)
			if !ok {
				rivenPending.Delete(key)
				return
			}
			// 双保险：定时任务可能尚未触发，这里再按时间戳判定过期
			if time.Since(req.at) > rivenPendingTimeout {
				rivenPending.Delete(key)
				return
			}
			urls := messageImageURLs(ctx)
			if len(urls) == 0 {
				return // 非图片消息：保留等待状态
			}
			clearRivenPending(key)
			logging.InfoPack("onebot.riven", "用户 %d 补发紫卡截图，开始分析", ctx.Event.UserID)
			registry.replyRivenAnalyse(ctx, urls)
		})
}

// replyRivenAnalyse 执行紫卡分析并回复图片（多图逐张分析后合并为一张趋势图）。
func (registry *CommandRegistry) replyRivenAnalyse(ctx *zero.Ctx, urls []string) {
	engine, err := ocr.Ready()
	if err != nil {
		if errors.Is(err, ocr.ErrNotReady) {
			_ = ReplyText(ctx, "识别服务准备中（模型校验/下载），请稍后再试")
		} else {
			_ = ReplyText(ctx, "识别服务暂不可用，请稍后再试")
			logging.WarnPack("onebot.riven", "OCR 引擎不可用: %v", err)
		}
		return
	}

	calculator := rivenanalyse.NewCalculator(database.DB)
	var models []*draw.RivenAnalyseTrend
	for _, url := range urls {
		img, err := downloadRivenImage(url)
		if err != nil {
			logging.WarnPack("onebot.riven", "下载紫卡截图失败（%s）: %v", url, err)
			continue
		}
		results, err := engine.Recognize(img)
		if err != nil {
			logging.WarnPack("onebot.riven", "OCR 识别失败: %v", err)
			continue
		}
		lines := make([]string, 0, len(results))
		for _, r := range results {
			lines = append(lines, r.Text)
		}
		models = append(models, calculator.Analyse(lines)...)
	}

	if len(models) == 0 {
		_ = ReplyText(ctx, "未能从截图中识别到紫卡信息：请确认截图包含清晰的武器名与属性词条（文字完整、无遮挡），重新截图后再试")
		return
	}
	data := draw.DrawRivenAnalyseTrend(models)
	if len(data) == 0 {
		_ = ReplyText(ctx, "紫卡分析图生成失败，请稍后再试")
		return
	}
	if err := ReplyImage(ctx, data); err != nil {
		logging.WarnPack("onebot.riven", "发送紫卡分析图失败: %v", err)
	}
}

// messageImageURLs 提取消息段中的图片 URL（优先 url 字段，回退 file；仅接受 http(s) 链接）。
func messageImageURLs(ctx *zero.Ctx) []string {
	if ctx == nil || ctx.Event == nil {
		return nil
	}
	var urls []string
	for _, seg := range ctx.Event.Message {
		if seg.Type != "image" {
			continue
		}
		u := seg.Data["url"]
		if u == "" {
			u = seg.Data["file"]
		}
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			urls = append(urls, u)
		}
	}
	return urls
}

// downloadRivenImage 下载并解码消息图片。
func downloadRivenImage(url string) (image.Image, error) {
	client := &http.Client{Timeout: rivenImageTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	img, _, err := image.Decode(resp.Body)
	return img, err
}
