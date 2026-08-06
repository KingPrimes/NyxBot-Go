// Package warframe 提供 Warframe 数据层：WorldState 轮询缓存、仲裁三级缓存、
// 市场 API 封装与官方导出文件导入器。
package warframe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// worldStateURL 官方 WorldState 远程 API。
const worldStateURL = "https://api.warframe.com/cdn/worldState.php"

// WorldState 轮询参数，对齐 Java TaskWarframeStatus 常量。
const (
	pollMinInterval  = 2 * time.Minute  // 最小轮询间隔
	pollMaxInterval  = 10 * time.Minute // 最大轮询间隔
	pollBuffer       = 30 * time.Second // 最早过期时间前的缓冲
	pollSmoothAfter  = 3                // 连续命中最小间隔 N 次后平滑为 5 分钟
	pollSmoothTo     = 5 * time.Minute  // 平滑后的间隔
	statusRetryCount = 3                // 轮询失败重试次数
	statusRetryWait  = 10 * time.Second // 重试间隔
)

// WorldStateCache 持有最新 WorldState 原始 JSON 与解析结果，支持并发读取。
type WorldStateCache struct {
	mu      sync.RWMutex
	raw     []byte                    // 最近一次成功拉取的原始 JSON（持久化到 ./data/status）
	state   *modelwarframe.WorldState // 反序列化结果
	fetched time.Time                 // 最近一次成功拉取时间
}

// NewWorldStateCache 创建空的世界状态缓存。
func NewWorldStateCache() *WorldStateCache {
	return &WorldStateCache{}
}

// Raw 返回最近一次成功拉取的原始 JSON 字节。
func (cache *WorldStateCache) Raw() []byte {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.raw
}

// State 返回最近一次反序列化结果；尚无数据时返回 nil。
func (cache *WorldStateCache) State() *modelwarframe.WorldState {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.state
}

// store 原子写入新状态并持久化原始 JSON 到磁盘。
func (cache *WorldStateCache) store(raw []byte, state *modelwarframe.WorldState) error {
	cache.mu.Lock()
	cache.raw = raw
	cache.state = state
	cache.fetched = time.Now()
	cache.mu.Unlock()
	return persistWorldState(raw)
}

// fetchWorldState 拉取并解析 WorldState，返回原始字节与解析结果。
func fetchWorldState(client *http.Client) ([]byte, *modelwarframe.WorldState, error) {
	request, err := http.NewRequest(http.MethodGet, worldStateURL, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("worldState.php returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}
	var state modelwarframe.WorldState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, nil, fmt.Errorf("parse worldState: %w", err)
	}
	return raw, &state, nil
}

// refresh 执行一次拉取（带重试），成功则更新缓存，返回错误供调度器决策。
func (cache *WorldStateCache) refresh(client *http.Client) error {
	var lastErr error
	for attempt := 1; attempt <= statusRetryCount; attempt++ {
		raw, state, err := fetchWorldState(client)
		if err == nil {
			if err := cache.store(raw, state); err != nil {
				logging.WarnPack("warframe.status", "persist worldState failed: %v", err)
			}
			return nil
		}
		lastErr = err
		if attempt < statusRetryCount {
			time.Sleep(statusRetryWait)
		}
	}
	return fmt.Errorf("fetch worldState after %d attempts: %w", statusRetryCount, lastErr)
}

// nextDelaySeconds 按最早过期时间计算下一次轮询延迟，对齐 Java calculateNextDelay。
// 无有效过期项时返回最大间隔；连续命中最小间隔 N 次后平滑为 5 分钟。
func (cache *WorldStateCache) nextDelaySeconds(consecutiveMin int) (int64, int) {
	expiries := collectExpiryTimestamps(cache.Raw())
	now := time.Now()
	var earliest time.Time
	found := false
	for _, expiry := range expiries {
		if expiry.After(now) && (!found || expiry.Before(earliest)) {
			earliest = expiry
			found = true
		}
	}
	if !found {
		return int64(pollMaxInterval / time.Second), 0
	}
	delay := earliest.Sub(now) - pollBuffer
	if delay < pollMinInterval {
		consecutiveMin++
		if consecutiveMin >= pollSmoothAfter {
			return int64(pollSmoothTo / time.Second), 0
		}
		return int64(pollMinInterval / time.Second), consecutiveMin
	}
	if delay > pollMaxInterval {
		delay = pollMaxInterval
	}
	return int64(delay / time.Second), 0
}

// collectExpiryTimestamps 收集 WorldState 原始 JSON 中各条目的过期时间。
// 仅覆盖本阶段实际使用的条目类型（裂缝/入侵/警报/突击/奸商/特惠/周期等）。
func collectExpiryTimestamps(raw []byte) []time.Time {
	if len(raw) == 0 {
		return nil
	}
	var probe struct {
		Fissures []struct {
			Expiry string `json:"expiry"`
		} `json:"fissures"`
		Invasions []struct {
			Expiry string `json:"expiry"`
		} `json:"invasions"`
		Alerts []struct {
			Expiry string `json:"expiry"`
		} `json:"alerts"`
		Sorties []struct {
			Expiry string `json:"expiry"`
		} `json:"sorties"`
		VoidTrader struct {
			Expiry string `json:"expiry"`
		} `json:"voidTrader"`
		DailyDeals []struct {
			Expiry string `json:"expiry"`
		} `json:"dailyDeals"`
		Cycles []struct {
			Expiry string `json:"expiry"`
		} `json:"cycles"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	results := make([]time.Time, 0, 16)
	appendTime := func(text string) {
		if text == "" {
			return
		}
		if parsed, err := time.Parse(time.RFC3339, text); err == nil {
			results = append(results, parsed)
		}
	}
	for _, item := range probe.Fissures {
		appendTime(item.Expiry)
	}
	for _, item := range probe.Invasions {
		appendTime(item.Expiry)
	}
	for _, item := range probe.Alerts {
		appendTime(item.Expiry)
	}
	for _, item := range probe.Sorties {
		appendTime(item.Expiry)
	}
	appendTime(probe.VoidTrader.Expiry)
	for _, item := range probe.DailyDeals {
		appendTime(item.Expiry)
	}
	for _, item := range probe.Cycles {
		appendTime(item.Expiry)
	}
	return results
}

// defaultCache 进程级默认 WorldState 缓存（供指令/接口直接读取）。
var defaultCache = NewWorldStateCache()

// DefaultWorldState 返回进程级默认 WorldState 缓存。
func DefaultWorldState() *WorldStateCache {
	return defaultCache
}

// RunWorldStatePolling 启动动态轮询调度，直到 ctx 取消。
// 首轮 5 秒后执行（对齐 Java startSchedule），之后按缓存 TTL 动态自调度。
func RunWorldStatePolling(ctx context.Context, client *http.Client) {
	if client == nil {
		client = http.DefaultClient
	}
	cache := defaultCache
	consecutiveMin := 0
	firstDelay := 5 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(firstDelay):
		}
		if err := cache.refresh(client); err != nil {
			logging.WarnPack("warframe.status", "worldState poll failed: %v", err)
			firstDelay = pollMaxInterval
			continue
		}
		delaySeconds, nextConsecutive := cache.nextDelaySeconds(consecutiveMin)
		consecutiveMin = nextConsecutive
		firstDelay = time.Duration(delaySeconds) * time.Second
		logging.DebugPack("warframe.status", "next worldState poll in %v", firstDelay)
	}
}

// persistWorldState 将原始 JSON 写入 ./data/status（对齐 Java WarframeCache 持久化，
// 避免反序列化再序列化体积膨胀）。
func persistWorldState(raw []byte) error {
	dataDir := filepath.Join(".", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dataDir, "status.tmp")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dataDir, "status"))
}

// LoadWorldStateFromDisk 启动时从 ./data/status 恢复上次快照；无文件时返回 nil。
func LoadWorldStateFromDisk() error {
	raw, err := os.ReadFile(filepath.Join(".", "data", "status"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var state modelwarframe.WorldState
	if err := json.Unmarshal(raw, &state); err != nil {
		logging.WarnPack("warframe.status", "parse persisted worldState failed: %v", err)
		return nil
	}
	defaultCache.store(raw, &state)
	return nil
}

// ParseWorldState 将原始 JSON 解析为 WorldState（供测试与指令复用）。
func ParseWorldState(raw []byte) (*modelwarframe.WorldState, error) {
	var state modelwarframe.WorldState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// TrimSpaceName 去除字符串首尾空白（供导入器统一清洗）。
func TrimSpaceName(value string) string {
	return strings.TrimSpace(value)
}
