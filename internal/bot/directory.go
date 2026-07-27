// Package bot 提供 Bot 在线目录以及管理员、黑白名单管理接口。
package bot

import (
	"sort"
	"strconv"
	"sync"
)

// Option 是 WebUI 下拉框使用的标签和值。
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type botEntry struct {
	uid      int64
	nickname string
	friends  []Option
	groups   []Option
}

// Directory 保存当前已连接 Bot 及其好友、群组快照。
// 阶段 7 的 OneBot 连接层通过本类型更新在线状态。
type Directory struct {
	mu   sync.RWMutex
	bots map[int64]botEntry
}

// DefaultDirectory 是 HTTP 路由使用的进程级在线 Bot 目录。
var DefaultDirectory = NewDirectory()

// NewDirectory 创建空的在线 Bot 目录。
func NewDirectory() *Directory {
	return &Directory{bots: make(map[int64]botEntry)}
}

// UpsertBot 新增或更新一个在线 Bot，保留其已有好友和群组快照。
func (d *Directory) UpsertBot(uid int64, nickname string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry := d.bots[uid]
	entry.uid = uid
	entry.nickname = nickname
	d.bots[uid] = entry
}

// RemoveBot 从在线目录移除指定 Bot。
func (d *Directory) RemoveBot(uid int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.bots, uid)
}

// SetFriends 更新指定在线 Bot 的好友选项快照，Bot 不存在时返回 false。
func (d *Directory) SetFriends(uid int64, friends []Option) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.bots[uid]
	if !ok {
		return false
	}
	entry.friends = cloneOptions(friends)
	d.bots[uid] = entry
	return true
}

// SetGroups 更新指定在线 Bot 的群组选项快照，Bot 不存在时返回 false。
func (d *Directory) SetGroups(uid int64, groups []Option) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.bots[uid]
	if !ok {
		return false
	}
	entry.groups = cloneOptions(groups)
	d.bots[uid] = entry
	return true
}

// Bots 返回按 Bot UID 排序的在线 Bot 选项。
func (d *Directory) Bots() []Option {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entries := make([]botEntry, 0, len(d.bots))
	for _, entry := range d.bots {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].uid < entries[j].uid })

	options := make([]Option, 0, len(entries))
	for _, entry := range entries {
		options = append(options, Option{Label: entry.nickname, Value: strconv.FormatInt(entry.uid, 10)})
	}
	return options
}

// Friends 返回指定 Bot 的好友快照和在线状态。
func (d *Directory) Friends(uid int64) ([]Option, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entry, ok := d.bots[uid]
	return cloneOptions(entry.friends), ok
}

// Groups 返回指定 Bot 的群组快照和在线状态。
func (d *Directory) Groups(uid int64) ([]Option, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entry, ok := d.bots[uid]
	return cloneOptions(entry.groups), ok
}

// Reset 清空在线目录，主要供连接层重启和测试隔离使用。
func (d *Directory) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bots = make(map[int64]botEntry)
}

func cloneOptions(options []Option) []Option {
	if options == nil {
		return []Option{}
	}
	return append([]Option(nil), options...)
}
