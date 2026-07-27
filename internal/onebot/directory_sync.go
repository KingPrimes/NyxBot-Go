package onebot

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"

	botdirectory "nyxbot-go/internal/bot"
	"nyxbot-go/internal/logging"
)

const (
	directoryDiscoveryInterval = 2 * time.Second
	directoryRefreshInterval   = 5 * time.Minute
	directoryRequestTimeout    = 15 * time.Second
	directoryRetryInterval     = 15 * time.Second
)

// BotDirectoryAPI 是同步在线 Bot 信息所需的最小 OneBot API 集合。
type BotDirectoryAPI interface {
	CallActionWithContext(context.Context, string, zero.Params) zero.APIResponse
}

// DirectorySyncer 定期把 ZeroBot 在线连接转换为 WebUI 使用的 Bot 目录快照。
type DirectorySyncer struct {
	directory *botdirectory.Directory
	trigger   chan int64

	mu       sync.Mutex
	lastSync map[int64]time.Time
}

// NewDirectorySyncer 创建在线目录同步器。
func NewDirectorySyncer(directory *botdirectory.Directory) *DirectorySyncer {
	if directory == nil {
		directory = botdirectory.DefaultDirectory
	}
	return &DirectorySyncer{
		directory: directory,
		trigger:   make(chan int64, 32),
		lastSync:  make(map[int64]time.Time),
	}
}

// Trigger 请求尽快同步指定 Bot；队列已满时由下一次轮询兜底。
func (s *DirectorySyncer) Trigger(botUID int64) {
	select {
	case s.trigger <- botUID:
	default:
	}
}

// Run 持续发现连接、刷新目录并移除离线 Bot，直到 ctx 被取消。
func (s *DirectorySyncer) Run(ctx context.Context) {
	ticker := time.NewTicker(directoryDiscoveryInterval)
	defer ticker.Stop()
	s.reconcile(ctx, false)
	for {
		select {
		case botUID := <-s.trigger:
			s.syncConnectedBot(ctx, botUID, true)
		case <-ticker.C:
			s.reconcile(ctx, false)
		case <-ctx.Done():
			return
		}
	}
}

// SyncBot 使用给定 API 立即同步一个 Bot，便于连接回调和黑盒测试复用。
func (s *DirectorySyncer) SyncBot(botUID int64, api BotDirectoryAPI) error {
	ctx, cancel := context.WithTimeout(context.Background(), directoryRequestTimeout)
	defer cancel()
	return s.syncBot(ctx, botUID, api)
}

func (s *DirectorySyncer) syncBot(ctx context.Context, botUID int64, api BotDirectoryAPI) error {
	if botUID <= 0 || api == nil {
		return errors.New("invalid Bot directory sync input")
	}

	loginResponse := api.CallActionWithContext(ctx, "get_login_info", zero.Params{})
	login := loginResponse.Data
	nickname := strconv.FormatInt(botUID, 10)
	var syncErrors []error
	if validAPIResponse(loginResponse) && validJSONObject(login) {
		if value := strings.TrimSpace(login.Get("nickname").String()); value != "" {
			nickname = value
		}
		s.directory.UpsertBot(botUID, nickname)
	} else {
		syncErrors = append(syncErrors, errors.New("get_login_info failed or returned no data"))
	}

	friendResponse := api.CallActionWithContext(ctx, "get_friend_list", zero.Params{})
	friendList := friendResponse.Data
	if validAPIResponse(friendResponse) && friendList.IsArray() {
		s.directory.SetFriends(botUID, friendOptions(friendList))
	} else {
		syncErrors = append(syncErrors, errors.New("get_friend_list failed or returned no array"))
	}

	groupResponse := api.CallActionWithContext(ctx, "get_group_list", zero.Params{})
	groupList := groupResponse.Data
	if validAPIResponse(groupResponse) && groupList.IsArray() {
		s.directory.SetGroups(botUID, groupOptions(groupList))
	} else {
		syncErrors = append(syncErrors, errors.New("get_group_list failed or returned no array"))
	}

	return errors.Join(syncErrors...)
}

func (s *DirectorySyncer) reconcile(ctx context.Context, force bool) {
	seen := make(map[int64]struct{})
	zero.RangeBot(func(botUID int64, _ *zero.Ctx) bool {
		seen[botUID] = struct{}{}
		s.syncConnectedBot(ctx, botUID, force)
		return ctx.Err() == nil
	})
	for _, option := range s.directory.Bots() {
		botUID, err := strconv.ParseInt(option.Value, 10, 64)
		if err != nil {
			continue
		}
		if _, online := seen[botUID]; !online {
			s.directory.RemoveBot(botUID)
			s.mu.Lock()
			delete(s.lastSync, botUID)
			s.mu.Unlock()
		}
	}
}

func (s *DirectorySyncer) syncConnectedBot(parent context.Context, botUID int64, force bool) {
	api := zero.GetBot(botUID)
	if api == nil {
		return
	}

	s.mu.Lock()
	last := s.lastSync[botUID]
	if !force && time.Since(last) < directoryRefreshInterval {
		s.mu.Unlock()
		return
	}
	// 先更新时间，防止慢接口期间重复发起同一轮同步。
	s.lastSync[botUID] = time.Now()
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(parent, directoryRequestTimeout)
	defer cancel()
	if err := s.syncBot(ctx, botUID, api); err != nil {
		s.mu.Lock()
		s.lastSync[botUID] = time.Now().Add(directoryRetryInterval - directoryRefreshInterval)
		s.mu.Unlock()
		logging.WarnPack("onebot.directory", "sync Bot %d directory partially failed: %v", botUID, err)
		return
	}
	logging.DebugPack("onebot.directory", "Bot %d directory synchronized", botUID)
}

func friendOptions(data gjson.Result) []botdirectory.Option {
	values := data.Array()
	options := make([]botdirectory.Option, 0, len(values))
	for _, value := range values {
		uid := value.Get("user_id").Int()
		if uid <= 0 {
			continue
		}
		label := strings.TrimSpace(value.Get("nickname").String())
		if label == "" {
			label = strings.TrimSpace(value.Get("remark").String())
		}
		if label == "" {
			label = strconv.FormatInt(uid, 10)
		}
		options = append(options, botdirectory.Option{Label: label, Value: strconv.FormatInt(uid, 10)})
	}
	return options
}

func groupOptions(data gjson.Result) []botdirectory.Option {
	values := data.Array()
	options := make([]botdirectory.Option, 0, len(values))
	for _, value := range values {
		uid := value.Get("group_id").Int()
		if uid <= 0 {
			continue
		}
		label := strings.TrimSpace(value.Get("group_name").String())
		if label == "" {
			label = strconv.FormatInt(uid, 10)
		}
		options = append(options, botdirectory.Option{Label: label, Value: strconv.FormatInt(uid, 10)})
	}
	return options
}

func validJSONObject(result gjson.Result) bool {
	return result.IsObject() && result.Raw != "" && result.Raw != "null"
}

func validAPIResponse(response zero.APIResponse) bool {
	return response.RetCode == 0 && (response.Status == "" || strings.EqualFold(response.Status, "ok"))
}
