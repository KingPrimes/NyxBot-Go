// 仲裁数据缓存，对应 Java NyxBot 的 ArbitrationCache
// 三级回退：内存缓存 → ./data/arbitration 文件（Base64 JSON）→ 远程 API
// 数据窗口裁剪：仅保留 expiry > now 且 activation < now+7d 的条目
package warframe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nyxbot-go/internal/logging"
)

// arbitrationURL 仲裁远程 API，请求头携带 User-Agent（对齐 Java ApiUrl.arbitrationPreList）。
const arbitrationURL = "https://wf.555590.xyz/api/arbys?days=30"

const (
	arbitrationFile      = "./data/arbitration" // 持久化文件（Base64 JSON）
	arbitrationSevenDays = 7 * 24 * time.Hour   // 窗口：activation 距今 7 天内
	arbitrationListLimit = 10                   // 前瞻列表条数
)

// Arbitration 仲裁条目，对齐 Java draw-image-plugin 的 Arbitration 模型。
type Arbitration struct {
	ID         string `json:"id"`         // 仲裁 ID
	Activation string `json:"activation"` // 开始时间（RFC3339）
	Expiry     string `json:"expiry"`     // 结束时间（RFC3339）
	Node       string `json:"node"`       // 节点
	Planet     string `json:"planet"`     // 行星
	Enemy      string `json:"enemy"`      // 敌人派系
	EnemyLv    int    `json:"enemyLv"`    // 敌人等级
	Type       string `json:"type"`       // 任务类型
	IsArchon   bool   `json:"isArchon"`   // 是否执刑官猎
	IsWorth    bool   `json:"isWorth"`    // 是否值得（有奖励）
}

// activationTime 解析开始时间；失败返回零值。
func (arb *Arbitration) activationTime() time.Time {
	parsed, _ := time.Parse(time.RFC3339, arb.Activation)
	return parsed
}

// expiryTime 解析结束时间；失败返回零值。
func (arb *Arbitration) expiryTime() time.Time {
	parsed, _ := time.Parse(time.RFC3339, arb.Expiry)
	return parsed
}

// ArbitrationCache 仲裁数据缓存，提供内存 → 文件 → API 三级回退读取。
type ArbitrationCache struct {
	mu     sync.RWMutex
	data   []Arbitration // 裁剪后的内存缓存
	client *http.Client
}

// NewArbitrationCache 创建仲裁缓存；client 为 nil 时使用 http.DefaultClient。
func NewArbitrationCache(client *http.Client) *ArbitrationCache {
	if client == nil {
		client = http.DefaultClient
	}
	return &ArbitrationCache{client: client}
}

// Init 启动时恢复仲裁数据：优先从文件恢复，文件无效则从 API 获取（对齐 Java init）。
func (cache *ArbitrationCache) Init() error {
	fromFile := cache.loadFromFile()
	if len(fromFile) > 0 && hasValidArbitration(fromFile) {
		cache.setMemoryCache(pruneArbitration(fromFile))
		logging.InfoPack("warframe.arbitration", "arbitration restored from file (%d entries)", len(cache.snapshot()))
		return nil
	}
	_, err := cache.reload()
	return err
}

// reload 强制从远程 API 重新获取仲裁数据（对齐 Java reloadArbitration）。
func (cache *ArbitrationCache) reload() ([]Arbitration, error) {
	full, err := fetchArbitrations(cache.client)
	if err != nil {
		logging.WarnPack("warframe.arbitration", "fetch arbitration failed: %v", err)
		return nil, err
	}
	if len(full) == 0 {
		logging.WarnPack("warframe.arbitration", "arbitration API returned empty data")
		return nil, errors.New("arbitration API returned empty data")
	}
	if err := cache.persistToFile(full); err != nil {
		logging.WarnPack("warframe.arbitration", "persist arbitration failed: %v", err)
	}
	pruned := pruneArbitration(full)
	cache.setMemoryCache(pruned)
	logging.InfoPack("warframe.arbitration", "arbitration refreshed: %d full, %d pruned", len(full), len(pruned))
	return pruned, nil
}

// GetArbitration 返回当前正在进行的仲裁（最近到期的一条）。
func (cache *ArbitrationCache) GetArbitration() (Arbitration, bool) {
	list := cache.load()
	now := time.Now()
	var best *Arbitration
	for index := range list {
		expiry := list[index].expiryTime()
		if expiry.After(now) && (best == nil || expiry.Sub(now) < best.expiryTime().Sub(now)) {
			best = &list[index]
		}
	}
	if best == nil {
		return Arbitration{}, false
	}
	return *best, true
}

// GetArbitrationList 返回有价值的未来仲裁列表（前瞻 10 条）。
func (cache *ArbitrationCache) GetArbitrationList() []Arbitration {
	list := cache.load()
	now := time.Now()
	results := make([]Arbitration, 0, arbitrationListLimit)
	for _, arb := range list {
		if !arb.IsWorth {
			continue
		}
		if arb.activationTime().After(now) {
			results = append(results, arb)
		}
		if len(results) >= arbitrationListLimit {
			break
		}
	}
	return results
}

// snapshot 返回当前内存缓存副本（测试与日志用）。
func (cache *ArbitrationCache) snapshot() []Arbitration {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return append([]Arbitration(nil), cache.data...)
}

// load 三级回退读取：内存 → 文件 → API。
func (cache *ArbitrationCache) load() []Arbitration {
	if cached := cache.snapshot(); hasValidArbitration(cached) {
		return cached
	}
	fromFile := cache.loadFromFile()
	if hasValidArbitration(fromFile) {
		pruned := pruneArbitration(fromFile)
		cache.setMemoryCache(pruned)
		return pruned
	}
	pruned, err := cache.reload()
	if err != nil {
		return nil
	}
	return pruned
}

func (cache *ArbitrationCache) setMemoryCache(list []Arbitration) {
	cache.mu.Lock()
	cache.data = list
	cache.mu.Unlock()
}

// hasValidArbitration 判断列表是否包含未过期条目。
func hasValidArbitration(list []Arbitration) bool {
	now := time.Now()
	for _, arb := range list {
		if arb.expiryTime().After(now) {
			return true
		}
	}
	return false
}

// pruneArbitration 裁剪窗口：expiry > now 且 activation < now+7d。
func pruneArbitration(full []Arbitration) []Arbitration {
	now := time.Now()
	cutoff := now.Add(arbitrationSevenDays)
	results := make([]Arbitration, 0, len(full))
	for _, arb := range full {
		if arb.expiryTime().After(now) && arb.activationTime().Before(cutoff) {
			results = append(results, arb)
		}
	}
	return results
}

// loadFromFile 从 ./data/arbitration 读取 Base64 JSON；失败返回 nil。
func (cache *ArbitrationCache) loadFromFile() []Arbitration {
	raw, err := os.ReadFile(arbitrationFile)
	if err != nil {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		logging.WarnPack("warframe.arbitration", "decode arbitration file failed: %v", err)
		return nil
	}
	var list []Arbitration
	if err := json.Unmarshal(decoded, &list); err != nil {
		logging.WarnPack("warframe.arbitration", "parse arbitration file failed: %v", err)
		return nil
	}
	return list
}

// persistToFile 将完整数据 Base64 编码后写入文件（对齐 Java persistToFile）。
func (cache *ArbitrationCache) persistToFile(list []Arbitration) error {
	encoded, err := json.Marshal(list)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(arbitrationFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(arbitrationFile, []byte(base64.StdEncoding.EncodeToString(encoded)), 0o644)
}

// fetchArbitrations 从远程 API 拉取仲裁列表（对齐 Java ApiUrl.arbitrationPreList）。
// 网络错误与 429/5xx 自动重试 3 次（指数退避）。
func fetchArbitrations(client *http.Client) ([]Arbitration, error) {
	headers := map[string]string{
		"User-Agent": fmt.Sprintf("NyxBot/%s", userAgentVersion()),
		"Accept":     "application/json",
	}
	body, err := doRequestWithRetry(context.Background(), client, http.MethodGet, arbitrationURL, headers)
	if err != nil {
		return nil, err
	}
	var list []Arbitration
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse arbitration response: %w", err)
	}
	return list, nil
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

// DefaultArbitration 返回进程级默认仲裁缓存，首次调用时懒初始化为文件或远程数据。
// 初始化失败仅导致空数据（指令会提示暂无仲裁），不阻塞调用方。
// 注意：首次调用会同步尝试初始化（含网络重试），最坏可能阻塞数秒；如需预热请提前调用。
func DefaultArbitration() *ArbitrationCache {
	defaultArbitrationOnce.Do(func() {
		cache := NewArbitrationCache(&http.Client{Timeout: 15 * time.Second})
		if err := cache.Init(); err != nil {
			logging.WarnPack("warframe.arbitration", "default arbitration init failed: %v", err)
		}
		defaultArbitration = cache
	})
	return defaultArbitration
}
