// Package onebot 集成 OneBot 11 与 ZeroBot，提供同端口反向 WebSocket、
// 正向 WebSocket、在线目录同步、指令权限和基础消息处理能力。
package onebot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RomiChan/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"

	botdirectory "nyxbot-go/internal/bot"
	"nyxbot-go/internal/logging"
)

const handshakeTimeout = 10 * time.Second

var (
	errReverseDriverClosed = errors.New("reverse WebSocket driver is closed")
	errCallerQueueFull     = errors.New("reverse WebSocket connection queue is full")
)

var reverseUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// ReverseDriver 是挂载到 Gin 主服务的 ZeroBot 反向 WebSocket Driver。
// 它保留请求路径，避免 ZeroBot 内置 WSServer 只监听独立端口的问题。
type ReverseDriver struct {
	accessToken string
	directory   *botdirectory.Directory
	callers     chan *wsCaller
	done        chan struct{}
	closeOnce   sync.Once

	mu        sync.Mutex
	active    map[int64]*wsCaller
	onConnect func(int64)
	closed    bool
}

// NewReverseDriver 创建反向 WebSocket Driver。
func NewReverseDriver(waitN int, accessToken string, directory *botdirectory.Directory) *ReverseDriver {
	if waitN <= 0 {
		waitN = 16
	}
	if directory == nil {
		directory = botdirectory.DefaultDirectory
	}
	return &ReverseDriver{
		accessToken: accessToken,
		directory:   directory,
		callers:     make(chan *wsCaller, waitN),
		done:        make(chan struct{}),
		active:      make(map[int64]*wsCaller),
	}
}

// SetConnectHook 设置 Bot 完成握手后的回调，用于立即同步好友和群组目录。
func (d *ReverseDriver) SetConnectHook(hook func(int64)) {
	d.mu.Lock()
	d.onConnect = hook
	d.mu.Unlock()
}

// Register 在 Gin 路由上注册反向 WebSocket 端点。
func (d *ReverseDriver) Register(routes gin.IRoutes, path string) {
	routes.GET(normalizeWebSocketPath(path), d.handleUpgrade)
}

// Connect 实现 zero.Driver。HTTP 监听由 Gin 管理，因此此处无需另起 listener。
func (d *ReverseDriver) Connect() {
	logging.InfoPack("onebot.ws", "reverse WebSocket driver is ready")
}

// Listen 实现 zero.Driver，持续把连接收到的 OneBot 事件交给 ZeroBot。
func (d *ReverseDriver) Listen(handler func([]byte, zero.APICaller)) {
	for {
		select {
		case caller := <-d.callers:
			if caller != nil {
				go caller.listen(handler)
			}
		case <-d.done:
			return
		}
	}
}

// Close 关闭反向 Driver 当前持有的所有连接。
func (d *ReverseDriver) Close() error {
	d.closeOnce.Do(func() {
		d.mu.Lock()
		d.closed = true
		close(d.done)
		callers := make([]*wsCaller, 0, len(d.active))
		for _, caller := range d.active {
			callers = append(callers, caller)
		}
		d.mu.Unlock()
		for _, caller := range callers {
			caller.close()
		}
	})
	return nil
}

func (d *ReverseDriver) handleUpgrade(c *gin.Context) {
	status := checkAccess(c.Request, d.accessToken)
	if status != http.StatusOK {
		logging.WarnPack("onebot.ws", "rejected WebSocket request from %s: authentication failed (%d)", c.Request.RemoteAddr, status)
		c.Status(status)
		return
	}

	conn, err := reverseUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logging.WarnPack("onebot.ws", "upgrade WebSocket request failed: %v", err)
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	var handshake struct {
		SelfID int64 `json:"self_id"`
	}
	if err := conn.ReadJSON(&handshake); err != nil || handshake.SelfID <= 0 {
		if err == nil {
			err = errors.New("handshake does not contain a valid self_id")
		}
		logging.WarnPack("onebot.ws", "OneBot handshake failed: %v", err)
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	caller := newWSCaller(d, conn, handshake.SelfID)
	previous, hook, err := d.activateCaller(caller)
	if err != nil {
		if errors.Is(err, errCallerQueueFull) {
			logging.WarnPack("onebot.ws", "connection queue is full; closing OneBot %d", handshake.SelfID)
		}
		caller.close()
		return
	}
	if previous != nil {
		previous.close()
	}
	logging.InfoPack("onebot.ws", "OneBot connected: %d", handshake.SelfID)
	if hook != nil {
		go hook(handshake.SelfID)
	}
}

func (d *ReverseDriver) activateCaller(caller *wsCaller) (*wsCaller, func(int64), error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, nil, errReverseDriverClosed
	}
	select {
	case d.callers <- caller:
	default:
		return nil, nil, errCallerQueueFull
	}
	previous := d.active[caller.selfID]
	d.active[caller.selfID] = caller
	zero.APICallers.Store(caller.selfID, caller)
	d.directory.UpsertBot(caller.selfID, strconv.FormatInt(caller.selfID, 10))
	return previous, d.onConnect, nil
}

func (d *ReverseDriver) removeCaller(caller *wsCaller) {
	d.mu.Lock()
	if d.active[caller.selfID] != caller {
		d.mu.Unlock()
		return
	}
	delete(d.active, caller.selfID)
	zero.APICallers.Delete(caller.selfID)
	d.directory.RemoveBot(caller.selfID)
	d.mu.Unlock()

	logging.WarnPack("onebot.ws", "OneBot disconnected: %d", caller.selfID)
}

func checkAccess(request *http.Request, token string) int {
	if token == "" {
		return http.StatusOK
	}
	auth := request.Header.Get("Authorization")
	if auth == "" {
		auth = request.URL.Query().Get("access_token")
	} else if _, value, ok := strings.Cut(auth, " "); ok {
		auth = value
	}
	auth = strings.TrimSpace(auth)
	switch auth {
	case token:
		return http.StatusOK
	case "":
		return http.StatusUnauthorized
	default:
		return http.StatusForbidden
	}
}

func normalizeWebSocketPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/ws/shiro"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

type callResult struct {
	response zero.APIResponse
	err      error
}

type wsCaller struct {
	driver *ReverseDriver
	conn   *websocket.Conn
	selfID int64
	seq    uint64

	writeMu   sync.Mutex
	pending   sync.Map
	closed    chan struct{}
	closeOnce sync.Once
}

func newWSCaller(driver *ReverseDriver, conn *websocket.Conn, selfID int64) *wsCaller {
	return &wsCaller{
		driver: driver,
		conn:   conn,
		selfID: selfID,
		closed: make(chan struct{}),
	}
}

func (caller *wsCaller) listen(handler func([]byte, zero.APICaller)) {
	defer caller.close()
	for {
		messageType, payload, err := caller.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		parsed := gjson.ParseBytes(payload)
		if echo := parsed.Get("echo"); echo.Exists() {
			caller.deliverResponse(echo.Uint(), parsed)
			continue
		}
		if parsed.Get("meta_event_type").String() == "heartbeat" {
			continue
		}
		if handler != nil {
			handler(payload, caller)
		}
	}
}

func (caller *wsCaller) deliverResponse(echo uint64, parsed gjson.Result) {
	value, ok := caller.pending.LoadAndDelete(echo)
	if !ok {
		return
	}
	message := parsed.Get("message").String()
	if message == "" {
		message = parsed.Get("msg").String()
	}
	value.(chan callResult) <- callResult{response: zero.APIResponse{
		Status:  parsed.Get("status").String(),
		Data:    parsed.Get("data"),
		Message: message,
		Wording: parsed.Get("wording").String(),
		RetCode: parsed.Get("retcode").Int(),
		Echo:    echo,
	}}
}

// CallAPI 实现 zero.APICaller，通过 echo 关联请求与响应。
func (caller *wsCaller) CallAPI(ctx context.Context, request zero.APIRequest) (zero.APIResponse, error) {
	select {
	case <-caller.closed:
		return zero.APIResponse{}, io.ErrClosedPipe
	default:
	}

	echo := atomic.AddUint64(&caller.seq, 1)
	request.Echo = echo
	waiter := make(chan callResult, 1)
	caller.pending.Store(echo, waiter)
	defer caller.pending.Delete(echo)

	caller.writeMu.Lock()
	err := caller.conn.WriteJSON(&request)
	caller.writeMu.Unlock()
	if err != nil {
		return zero.APIResponse{}, err
	}

	select {
	case result := <-waiter:
		return result.response, result.err
	case <-ctx.Done():
		return zero.APIResponse{}, ctx.Err()
	case <-caller.closed:
		return zero.APIResponse{}, io.ErrClosedPipe
	}
}

func (caller *wsCaller) close() {
	caller.closeOnce.Do(func() {
		close(caller.closed)
		_ = caller.conn.Close()
		caller.pending.Range(func(key, value any) bool {
			if current, ok := caller.pending.LoadAndDelete(key); ok {
				current.(chan callResult) <- callResult{err: io.ErrClosedPipe}
			}
			return true
		})
		caller.driver.removeCaller(caller)
	})
}
