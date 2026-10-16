// 仲裁数据缓存，对应 Java NyxBot 的 ArbitrationCache。
// 数据源为 browse.wf/arbys.txt（逐行 "unix秒,节点ID" 的官方导出）：启动时后台拉取并落
// 本地文件 ./data/arbys.txt，消费路径只读本地文件（运行期不触网），本地文件也不可用时
// 返回空列表，由「更新仲裁」指令补救。
// 窗口裁剪：仅保留 expiry > now 且 activation < now+30d 的条目（解析阶段即丢弃，不建全量切片）。
package warframe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
	"nyxbot-go/internal/logging"
)

// arbitrationURL 仲裁原始数据地址（browse.wf 官方导出，纯文本逐行 "unix秒,节点ID"，无表头）。
const arbitrationURL = "https://browse.wf/arbys.txt"

const (
	arbitrationFile      = "./data/arbys.txt"  // 本地缓存文件（远程原文）
	arbitrationWindow    = 30 * 24 * time.Hour // 窗口：activation 距今 30 天内
	arbitrationDuration  = 3600                // 单场仲裁时长（秒，官方排期间隔恒为 3600）
	arbitrationListLimit = 10                  // 前瞻列表条数
)

// Arbitration 仲裁条目，只承载数据源的两列与由此推导的到期时间、派系。
// 节点名（含星系）/ 任务类型 / 评级等显示值一律不落库，由 nodes 表在转换时补齐
// （见 ArbitrationToDraw）。
type Arbitration struct {
	Activation int64              // 开始时间（Unix 秒，arbys.txt 第一列）
	NodeKey    string             // 节点 SolNode ID（arbys.txt 第二列）
	Expiry     int64              // 结束时间（= Activation + arbitrationDuration）
	Faction    drawplugin.Faction // 派系枚举（由 nodes 表派生）
}

// ArbitrationCache 仲裁数据缓存：内存缓存 + 本地文件两级，消费路径不触网。
type ArbitrationCache struct {
	mu     sync.RWMutex
	data   []Arbitration // 内存缓存（已按窗口裁剪）
	client *http.Client
}

// NewArbitrationCache 创建仲裁缓存；client 为 nil 时使用 http.DefaultClient。
func NewArbitrationCache(client *http.Client) *ArbitrationCache {
	if client == nil {
		client = http.DefaultClient
	}
	return &ArbitrationCache{client: client}
}

// Reload 强制重拉仲裁数据：拉取 → 覆写本地文件 → 刷新内存缓存。
// 供「更新仲裁」指令与 HTTP 接口调用（见迁移计划 §8.2）。
func (cache *ArbitrationCache) Reload() error {
	return cache.reload()
}

// reload 拉取远程原文并刷新缓存。拉取失败或窗口内无数据时返回错误，且不覆写本地文件，
// 避免把错误页写进缓存。
func (cache *ArbitrationCache) reload() error {
	raw, err := fetchArbys(cache.client)
	if err != nil {
		logging.WarnPack("warframe.arbitration", "fetch arbys.txt failed: %v", err)
		return err
	}
	list := parseArbys(raw)
	if len(list) == 0 {
		logging.WarnPack("warframe.arbitration", "arbys.txt has no entry in window")
		return errors.New("arbys.txt has no entry in window")
	}
	if err := cache.writeLocalFile(raw); err != nil {
		logging.WarnPack("warframe.arbitration", "persist arbys.txt failed: %v", err)
	}
	cache.setMemoryCache(list)
	logging.InfoPack("warframe.arbitration", "arbitration refreshed: %d entries in window", len(list))
	return nil
}

// GetArbitration 返回当前正在进行的仲裁（最近到期的一条）。
func (cache *ArbitrationCache) GetArbitration() (Arbitration, bool) {
	list := cache.load()
	now := time.Now().Unix()
	var best *Arbitration
	for index := range list {
		expiry := list[index].Expiry
		if expiry > now && (best == nil || expiry < best.Expiry) {
			best = &list[index]
		}
	}
	if best == nil {
		return Arbitration{}, false
	}
	return *best, true
}

// GetArbitrationList 返回值得参与的未来仲裁列表（按分级阈值过滤，上限 arbitrationListLimit 条）。
func (cache *ArbitrationCache) GetArbitrationList() []Arbitration {
	list := cache.load()
	now := time.Now().Unix()
	results := make([]Arbitration, 0, arbitrationListLimit)
	for _, arb := range list {
		if !IsWorthTier(ArbitrationTier(arb.NodeKey)) {
			continue
		}
		if arb.Activation > now {
			results = append(results, arb)
		}
		if len(results) >= arbitrationListLimit {
			break
		}
	}
	return results
}

// snapshot 返回当前内存缓存副本（日志用）。
func (cache *ArbitrationCache) snapshot() []Arbitration {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return append([]Arbitration(nil), cache.data...)
}

// load 消费路径读取：内存缓存 → 本地文件 → 空列表。全程不触网，
// 首启拉取未完成或失败时退化为读本地文件，文件也不可用则返回空列表。
func (cache *ArbitrationCache) load() []Arbitration {
	if cached := cache.snapshot(); len(cached) > 0 {
		return cached
	}
	list := cache.readLocalFile()
	if len(list) == 0 {
		logging.WarnPack("warframe.arbitration", "no arbitration data available (local file missing or out of window)")
		return nil
	}
	cache.setMemoryCache(list)
	return list
}

// readLocalFile 读取并解析本地缓存文件；文件缺失或窗口内无数据时返回 nil。
func (cache *ArbitrationCache) readLocalFile() []Arbitration {
	raw, err := os.ReadFile(arbitrationFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.WarnPack("warframe.arbitration", "read %s failed: %v", arbitrationFile, err)
		}
		return nil
	}
	return parseArbys(raw)
}

func (cache *ArbitrationCache) setMemoryCache(list []Arbitration) {
	cache.mu.Lock()
	cache.data = list
	cache.mu.Unlock()
}

// parseArbys 解析 arbys.txt 原文，边解析边按窗口过滤：畸形行（空行、缺列、时间戳非数字）
// 直接跳过，已过期与超出前瞻窗口的行不入结果，不构建全量切片。
// 派系按节点去重查询 nodes 表，避免逐行查库。
func parseArbys(raw []byte) []Arbitration {
	now := time.Now()
	nowUnix := now.Unix()
	cutoffUnix := now.Add(arbitrationWindow).Unix()

	results := make([]Arbitration, 0, 64)
	factions := make(map[string]drawplugin.Faction)
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		comma := strings.IndexByte(line, ',')
		if comma <= 0 || comma == len(line)-1 {
			continue
		}
		activation, err := strconv.ParseInt(strings.TrimSpace(line[:comma]), 10, 64)
		if err != nil {
			continue
		}
		nodeKey := strings.TrimSpace(line[comma+1:])
		if nodeKey == "" {
			continue
		}
		expiry := activation + arbitrationDuration
		if expiry <= nowUnix || activation >= cutoffUnix {
			continue
		}
		faction, ok := factions[nodeKey]
		if !ok {
			faction = nodeFactionByKey(nodeKey)
			factions[nodeKey] = faction
		}
		results = append(results, Arbitration{
			Activation: activation,
			NodeKey:    nodeKey,
			Expiry:     expiry,
			Faction:    faction,
		})
	}
	return results
}

// writeLocalFile 将远程原文写入本地缓存文件。先写临时文件再改名，
// 避免并发消费读到写了一半的内容。
func (cache *ArbitrationCache) writeLocalFile(raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(arbitrationFile), 0o755); err != nil {
		return err
	}
	tmp := arbitrationFile + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, arbitrationFile)
}

// fetchArbys 从 browse.wf 拉取 arbys.txt 原文。
// 网络错误与 429/5xx 自动重试 3 次（指数退避）。
func fetchArbys(client *http.Client) ([]byte, error) {
	headers := map[string]string{
		"User-Agent": fmt.Sprintf("NyxBot/%s", userAgentVersion()),
	}
	return doRequestWithRetry(context.Background(), client, http.MethodGet, arbitrationURL, headers)
}

// userAgentVersion 返回进程版本号；未注入时回退 "dev"。
var userAgentVersion = func() string {
	return "dev"
}

// SetUserAgentVersion 供 main 注入版本号（对齐 Java 的 NyxBot/{jarVersion}）。
func SetUserAgentVersion(version string) {
	userAgentVersion = func() string { return version }
}

var (
	defaultArbitrationOnce sync.Once
	defaultArbitration     *ArbitrationCache
)

// DefaultArbitration 返回进程级默认仲裁缓存。首次调用在 sync.Once 内构建缓存并启动
// 后台拉取（同步路径零网络，不阻塞调用方）；拉取失败时消费路径读本地文件，
// 文件也不可用则返回空列表（指令提示暂无仲裁），由「更新仲裁」指令补救。
func DefaultArbitration() *ArbitrationCache {
	defaultArbitrationOnce.Do(func() {
		cache := NewArbitrationCache(&http.Client{Timeout: 15 * time.Second})
		defaultArbitration = cache
		go func() {
			if err := cache.Reload(); err != nil {
				logging.WarnPack("warframe.arbitration", "default arbitration init failed: %v", err)
			}
		}()
	})
	return defaultArbitration
}
