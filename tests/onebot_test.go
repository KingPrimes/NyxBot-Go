package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RomiChan/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm"

	botdirectory "nyxbot-go/internal/bot"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	modelbot "nyxbot-go/internal/model/bot"
	modelsystem "nyxbot-go/internal/model/system"
	"nyxbot-go/internal/onebot"
)

// TestReverseDriver 验证反向 WS 的鉴权、握手、echo 分发、心跳过滤和断线清理。
func TestReverseDriver(t *testing.T) {
	gin.SetMode(gin.TestMode)
	directory := botdirectory.NewDirectory()
	driver := onebot.NewReverseDriver(4, "secret", directory)
	events := make(chan string, 2)
	go driver.Listen(func(payload []byte, _ zero.APICaller) {
		events <- string(payload)
	})
	t.Cleanup(func() {
		_ = driver.Close()
		zero.APICallers.Delete(123456)
		zero.APICallers.Delete(123457)
	})

	router := gin.New()
	driver.Register(router, "/ws/shiro")
	server := httptest.NewServer(router)
	defer server.Close()

	assertHTTPStatus(t, server.URL+"/ws/shiro", http.StatusUnauthorized)
	assertHTTPStatus(t, server.URL+"/ws/shiro?access_token=wrong", http.StatusForbidden)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/shiro"
	dialer := websocket.Dialer{}
	header := http.Header{"Authorization": []string{"Bearer secret"}}
	conn, response, err := dialer.Dial(wsURL, header)
	if err != nil {
		if response != nil {
			t.Fatalf("dial reverse WebSocket: %v (status %d)", err, response.StatusCode)
		}
		t.Fatalf("dial reverse WebSocket: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"self_id": int64(123456), "post_type": "meta_event"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(directory.Bots()) == 1 }, "Bot should enter online directory")

	apiResult := make(chan zero.APIResponse, 1)
	go func() {
		bot := zero.GetBot(123456)
		if bot == nil {
			apiResult <- zero.APIResponse{Message: "Bot not registered"}
			return
		}
		apiResult <- bot.CallActionWithContext(context.Background(), "get_login_info", zero.Params{})
	}()
	var request zero.APIRequest
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatalf("read API request: %v", err)
	}
	if request.Action != "get_login_info" || request.Echo == 0 {
		t.Fatalf("unexpected API request: %+v", request)
	}
	if err := conn.WriteJSON(map[string]any{
		"status": "ok", "retcode": 0, "echo": request.Echo,
		"data": map[string]any{"user_id": 123456, "nickname": "Nyx"},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-apiResult:
		if result.RetCode != 0 || result.Data.Get("nickname").String() != "Nyx" {
			t.Fatalf("unexpected API response: %+v data=%s", result, result.Data.Raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for echo response")
	}

	if err := conn.WriteJSON(map[string]any{"post_type": "meta_event", "meta_event_type": "heartbeat"}); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(map[string]any{"post_type": "message", "message_type": "private", "self_id": 123456, "user_id": 42, "raw_message": "帮助", "message": "帮助"}); err != nil {
		t.Fatal(err)
	}
	select {
	case payload := <-events:
		if !strings.Contains(payload, `"raw_message":"帮助"`) {
			t.Fatalf("unexpected event payload: %s", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OneBot event")
	}

	_ = conn.Close()
	waitFor(t, func() bool { return len(directory.Bots()) == 0 }, "disconnected Bot should leave online directory")

	queryConn, response, err := dialer.Dial(wsURL+"?access_token=secret", nil)
	if err != nil {
		if response != nil {
			t.Fatalf("dial query-token WebSocket: %v (status %d)", err, response.StatusCode)
		}
		t.Fatalf("dial query-token WebSocket: %v", err)
	}
	if err := queryConn.WriteJSON(map[string]any{"self_id": int64(123457), "post_type": "meta_event"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(directory.Bots()) == 1 }, "query access_token should be accepted")
	_ = queryConn.Close()
	waitFor(t, func() bool { return len(directory.Bots()) == 0 }, "query-token Bot should leave after disconnect")

	firstReplacement, _, err := dialer.Dial(wsURL+"?access_token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer firstReplacement.Close()
	if err := firstReplacement.WriteJSON(map[string]any{"self_id": int64(123458)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return zero.GetBot(123458) != nil }, "first duplicate Bot connection should become active")
	secondReplacement, _, err := dialer.Dial(wsURL+"?access_token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer secondReplacement.Close()
	if err := secondReplacement.WriteJSON(map[string]any{"self_id": int64(123458)}); err != nil {
		t.Fatal(err)
	}
	_ = firstReplacement.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := firstReplacement.ReadMessage(); err == nil {
		t.Fatal("the previous connection should be closed after duplicate self_id replacement")
	}
	if len(directory.Bots()) != 1 {
		t.Fatalf("closing the replaced connection must not remove the new Bot: %+v", directory.Bots())
	}

	callContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	replacementResult := make(chan zero.APIResponse, 1)
	go func() {
		replacementResult <- zero.GetBot(123458).CallActionWithContext(callContext, "get_status", zero.Params{})
	}()
	_ = secondReplacement.SetReadDeadline(time.Now().Add(2 * time.Second))
	var replacementRequest zero.APIRequest
	if err := secondReplacement.ReadJSON(&replacementRequest); err != nil {
		t.Fatal(err)
	}
	if err := secondReplacement.WriteJSON(map[string]any{"status": "ok", "retcode": 0, "echo": replacementRequest.Echo, "data": map[string]any{"online": true}}); err != nil {
		t.Fatal(err)
	}
	if result := <-replacementResult; !result.Data.Get("online").Bool() {
		t.Fatalf("API caller should point at the replacement connection: %+v", result)
	}
	_ = secondReplacement.Close()
	waitFor(t, func() bool { return len(directory.Bots()) == 0 }, "replacement Bot should leave after disconnect")
	zero.APICallers.Delete(123458)
}

// TestReverseDriverCloseAndQueue 验证关闭竞态和连接队列满载不会破坏已有会话。
func TestReverseDriverCloseAndQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("queue full keeps existing Bot", func(t *testing.T) {
		directory := botdirectory.NewDirectory()
		driver := onebot.NewReverseDriver(1, "", directory)
		t.Cleanup(func() {
			_ = driver.Close()
			zero.APICallers.Delete(123459)
			zero.APICallers.Delete(123460)
		})
		router := gin.New()
		driver.Register(router, "/ws/shiro")
		server := httptest.NewServer(router)
		defer server.Close()
		dialer := websocket.Dialer{}
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/shiro"

		first, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer first.Close()
		if err := first.WriteJSON(map[string]any{"self_id": int64(123459)}); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return zero.GetBot(123459) != nil }, "first queued Bot should become active")

		second, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer second.Close()
		if err := second.WriteJSON(map[string]any{"self_id": int64(123460)}); err != nil {
			t.Fatal(err)
		}
		_ = second.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := second.ReadMessage(); err == nil {
			t.Fatal("queue-full connection should be closed")
		}
		bots := directory.Bots()
		if len(bots) != 1 || bots[0].Value != "123459" || zero.GetBot(123459) == nil {
			t.Fatalf("queue-full connection must not replace the existing Bot: %+v", bots)
		}
	})

	t.Run("close rejects pending handshake", func(t *testing.T) {
		directory := botdirectory.NewDirectory()
		driver := onebot.NewReverseDriver(1, "", directory)
		t.Cleanup(func() {
			_ = driver.Close()
			zero.APICallers.Delete(123461)
		})
		router := gin.New()
		driver.Register(router, "/ws/shiro")
		server := httptest.NewServer(router)
		defer server.Close()
		dialer := websocket.Dialer{}
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/shiro"
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err := driver.Close(); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(map[string]any{"self_id": int64(123461)}); err != nil {
			t.Fatal(err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Fatal("a handshake completed after Close should be rejected")
		}
		if len(directory.Bots()) != 0 || zero.GetBot(123461) != nil {
			t.Fatalf("closed driver must not publish pending handshakes: %+v", directory.Bots())
		}
	})
}

func assertHTTPStatus(t *testing.T, url string, expected int) {
	t.Helper()
	response, err := http.Get(url) //nolint:gosec // httptest URL only
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		t.Fatalf("GET %s: expected %d, got %d", url, expected, response.StatusCode)
	}
}

func waitFor(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

type fakeDirectoryAPI struct {
	login   gjson.Result
	friends gjson.Result
	groups  gjson.Result
	status  string
	retCode int64
}

func (api fakeDirectoryAPI) CallActionWithContext(_ context.Context, action string, _ zero.Params) zero.APIResponse {
	status := api.status
	if status == "" {
		status = "ok"
	}
	switch action {
	case "get_login_info":
		return zero.APIResponse{Status: status, RetCode: api.retCode, Data: api.login}
	case "get_friend_list":
		return zero.APIResponse{Status: status, RetCode: api.retCode, Data: api.friends}
	case "get_group_list":
		return zero.APIResponse{Status: status, RetCode: api.retCode, Data: api.groups}
	default:
		return zero.APIResponse{Status: "failed", RetCode: -1}
	}
}

// TestDirectorySyncer 验证在线 Bot、好友和群组响应被映射为 WebUI 下拉选项。
func TestDirectorySyncer(t *testing.T) {
	directory := botdirectory.NewDirectory()
	syncer := onebot.NewDirectorySyncer(directory)
	api := fakeDirectoryAPI{
		login:   gjson.Parse(`{"user_id":10001,"nickname":"Nyx"}`),
		friends: gjson.Parse(`[{"user_id":20002,"nickname":"Alice","remark":"A"}]`),
		groups:  gjson.Parse(`[{"group_id":30003,"group_name":"Tenno"}]`),
	}
	if err := syncer.SyncBot(10001, api); err != nil {
		t.Fatal(err)
	}
	if bots := directory.Bots(); len(bots) != 1 || bots[0].Label != "Nyx" || bots[0].Value != "10001" {
		t.Fatalf("unexpected Bot options: %+v", bots)
	}
	if friends, ok := directory.Friends(10001); !ok || len(friends) != 1 || friends[0].Label != "Alice" || friends[0].Value != "20002" {
		t.Fatalf("unexpected friend options: %+v online=%v", friends, ok)
	}
	if groups, ok := directory.Groups(10001); !ok || len(groups) != 1 || groups[0].Label != "Tenno" || groups[0].Value != "30003" {
		t.Fatalf("unexpected group options: %+v online=%v", groups, ok)
	}

	invalidLists := fakeDirectoryAPI{
		login:   gjson.Parse(`{"user_id":10001,"nickname":"Nyx 2"}`),
		friends: gjson.Parse(`null`),
		groups:  gjson.Parse(`null`),
	}
	if err := syncer.SyncBot(10001, invalidLists); err == nil {
		t.Fatal("invalid list responses should report a partial sync error")
	}
	friends, _ := directory.Friends(10001)
	groups, _ := directory.Groups(10001)
	if len(friends) != 1 || len(groups) != 1 {
		t.Fatalf("failed refresh must preserve previous snapshots: friends=%+v groups=%+v", friends, groups)
	}

	failedLists := fakeDirectoryAPI{
		login:   gjson.Parse(`{"user_id":10001,"nickname":"ignored"}`),
		friends: gjson.Parse(`[]`),
		groups:  gjson.Parse(`[]`),
		status:  "failed",
		retCode: 100,
	}
	if err := syncer.SyncBot(10001, failedLists); err == nil {
		t.Fatal("failed API status should report a sync error")
	}
	friends, _ = directory.Friends(10001)
	groups, _ = directory.Groups(10001)
	if len(friends) != 1 || len(groups) != 1 || directory.Bots()[0].Label != "Nyx 2" {
		t.Fatalf("failed API status must preserve all previous snapshots: bots=%+v friends=%+v groups=%+v", directory.Bots(), friends, groups)
	}
}

// TestOneBotAccessChecker 验证黑白名单优先级、艾特开关和动态管理员权限。
func TestOneBotAccessChecker(t *testing.T) {
	setupBotConfigDB(t)
	checker := onebot.NewAccessChecker(false)
	event := &zero.Event{SelfID: 10001, GroupID: 30001, UserID: 20001, Sender: &zero.User{Role: "member"}}
	if !checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("empty lists should allow normal users")
	}

	if err := database.DB.Create(&modelbot.GroupWhite{BotUID: 10001, GroupUID: 30001}).Error; err != nil {
		t.Fatal(err)
	}
	if !checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("matching whitelist should allow event")
	}
	event.GroupID = 30002
	if checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("whitelist-only mode should reject non-matching event")
	}

	clearBotLists(t)
	if err := database.DB.Create(&modelbot.GroupBlack{BotUID: 10001, GroupUID: 30001}).Error; err != nil {
		t.Fatal(err)
	}
	event.GroupID = 30001
	if checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("matching blacklist should reject event")
	}
	event.GroupID = 30002
	if !checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("blacklist-only mode should allow non-matching event")
	}

	clearBotLists(t)
	if err := database.DB.Create(&modelbot.GroupWhite{BotUID: 10001, GroupUID: 30001}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelbot.GroupBlack{BotUID: 10001, GroupUID: 30002}).Error; err != nil {
		t.Fatal(err)
	}
	event.GroupID = 30003
	if !checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("with both lists enabled, unrelated event should be allowed")
	}
	event.GroupID = 30002
	if checker.Allows(event, nyxbot.PermUser) {
		t.Fatal("with both lists enabled, blacklist should reject event")
	}

	clearBotLists(t)
	if err := database.DB.Create(&modelbot.BotAdmin{BotUID: 10001, AdminUID: 20001, Permissions: string(nyxbot.PermAdmin)}).Error; err != nil {
		t.Fatal(err)
	}
	event.GroupID = 30001
	if !checker.Allows(event, nyxbot.PermAdmin) || checker.Allows(event, nyxbot.PermSuperAdmin) {
		t.Fatal("database ADMIN should pass admin but not super-admin command")
	}
	database.DB.Model(&modelbot.BotAdmin{}).Where("bot_uid = ? AND admin_uid = ?", 10001, 20001).Update("permissions", string(nyxbot.PermSuperAdmin))
	if !checker.Allows(event, nyxbot.PermSuperAdmin) {
		t.Fatal("database SUPER_ADMIN should pass super-admin command")
	}

	prefixChecker := onebot.NewAccessChecker(true)
	event.IsToMe = false
	if prefixChecker.Allows(event, nyxbot.PermUser) {
		t.Fatal("group command without @ should be rejected when plugin_prefix is enabled")
	}
	event.IsToMe = true
	if !prefixChecker.Allows(event, nyxbot.PermUser) {
		t.Fatal("group command with @ should pass when plugin_prefix is enabled")
	}
	event.GroupID = 0
	event.IsToMe = false
	if !prefixChecker.Allows(event, nyxbot.PermUser) {
		t.Fatal("private command should not require @")
	}

	dynamicPrefix := false
	dynamicChecker := onebot.NewAccessCheckerWithProvider(func() bool { return dynamicPrefix })
	event.GroupID = 30001
	if !dynamicChecker.Allows(event, nyxbot.PermUser) {
		t.Fatal("disabled dynamic prefix should allow a group command without @")
	}
	dynamicPrefix = true
	if dynamicChecker.Allows(event, nyxbot.PermUser) {
		t.Fatal("updated dynamic prefix should immediately require @")
	}
}

func clearBotLists(t *testing.T) {
	t.Helper()
	for _, model := range []any{&modelbot.GroupWhite{}, &modelbot.ProveWhite{}, &modelbot.GroupBlack{}, &modelbot.ProveBlack{}, &modelbot.BotAdmin{}} {
		if err := database.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error; err != nil {
			t.Fatal(err)
		}
	}
}

type recordingCaller struct {
	mu       sync.Mutex
	requests []zero.APIRequest
}

func (caller *recordingCaller) snapshot() []zero.APIRequest {
	caller.mu.Lock()
	defer caller.mu.Unlock()
	return append([]zero.APIRequest(nil), caller.requests...)
}

func (caller *recordingCaller) CallAPI(_ context.Context, request zero.APIRequest) (zero.APIResponse, error) {
	caller.mu.Lock()
	caller.requests = append(caller.requests, request)
	caller.mu.Unlock()
	return zero.APIResponse{Status: "ok", Data: gjson.Parse(`{"message_id":99}`)}, nil
}

// TestProactiveMessageSending 验证主动群聊、私聊和图片消息使用正确 OneBot action。
func TestProactiveMessageSending(t *testing.T) {
	caller := &recordingCaller{}
	zero.APICallers.Store(10001, caller)
	t.Cleanup(func() { zero.APICallers.Delete(10001) })

	if err := onebot.SendGroupText(10001, 30001, "group"); err != nil {
		t.Fatal(err)
	}
	if err := onebot.SendPrivateText(10001, 20001, "private"); err != nil {
		t.Fatal(err)
	}
	if err := onebot.SendGroupImage(10001, 30001, []byte("png")); err != nil {
		t.Fatal(err)
	}
	if err := onebot.SendPrivateImage(10001, 20001, []byte("png")); err != nil {
		t.Fatal(err)
	}

	requests := caller.snapshot()
	if len(requests) != 4 {
		t.Fatalf("expected 4 API requests, got %d", len(requests))
	}
	expectedActions := []string{"send_group_msg", "send_private_msg", "send_group_msg", "send_private_msg"}
	for index, action := range expectedActions {
		if requests[index].Action != action {
			t.Fatalf("request %d: expected %s, got %s", index, action, requests[index].Action)
		}
	}
	for _, index := range []int{2, 3} {
		content, ok := requests[index].Params["message"].(message.Message)
		if !ok || len(content) != 1 || content[0].Type != "image" || !strings.HasPrefix(content[0].Data["file"], "base64://") {
			t.Fatalf("request %d should contain a base64 image: %#v", index, requests[index].Params["message"])
		}
	}
}

// TestCommandRegistry 验证帮助、系统信息、异常回复和 Java 兼容的插件日志状态。
func TestCommandRegistry(t *testing.T) {
	setupBotConfigDB(t)
	if err := database.DB.AutoMigrate(&modelsystem.LogInfo{}); err != nil {
		t.Fatal(err)
	}

	caller := &recordingCaller{}
	const botUID = int64(11001)
	zero.APICallers.Store(botUID, caller)
	previousConfig := zero.BotConfig
	zero.BotConfig = zero.Config{MaxProcessTime: 2 * time.Second}
	registry := onebot.NewCommandRegistry(false)
	if err := registry.SetHandler(nyxbot.CmdWfAlerts, func(*zero.Ctx, string) error {
		return errors.New("boom")
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		registry.Close()
		zero.BotConfig = previousConfig
		zero.APICallers.Delete(botUID)
	})

	bot := zero.GetBot(botUID)
	if bot == nil {
		t.Fatal("test Bot should be available")
	}
	echoPrivateMessage(t, bot, botUID, 21001, "帮助")
	waitFor(t, func() bool { return len(caller.snapshot()) >= 1 }, "help command should send a reply")
	requests := caller.snapshot()
	if text, ok := requests[0].Params["message"].(string); !ok || !strings.Contains(text, "NyxBot 指令") || !strings.Contains(text, helpDocumentURLForTest) {
		t.Fatalf("unexpected help reply: %#v", requests[0].Params["message"])
	}

	echoPrivateMessage(t, bot, botUID, 21001, "状态")
	waitFor(t, func() bool { return len(caller.snapshot()) >= 2 }, "system-info command should send a reply")
	requests = caller.snapshot()
	if text, ok := requests[1].Params["message"].(string); !ok || !strings.Contains(text, "Go go") || !strings.Contains(text, "运行时间") {
		t.Fatalf("unexpected system-info reply: %#v", requests[1].Params["message"])
	}

	echoPrivateMessage(t, bot, botUID, 21001, "警报")
	waitFor(t, func() bool { return len(caller.snapshot()) >= 3 }, "failed command should send an error reply")
	requests = caller.snapshot()
	if text, ok := requests[2].Params["message"].(string); !ok || !strings.Contains(text, "执行异常") || !strings.Contains(text, "boom") {
		t.Fatalf("unexpected command error reply: %#v", requests[2].Params["message"])
	}

	waitFor(t, func() bool {
		var count int64
		return database.DB.Model(&modelsystem.LogInfo{}).Count(&count).Error == nil && count == 3
	}, "commands should create operation logs")
	var logs []modelsystem.LogInfo
	if err := database.DB.Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	logsByCode := make(map[string]modelsystem.LogInfo, len(logs))
	for _, entry := range logs {
		logsByCode[entry.Code] = entry
	}
	helpLog, ok := logsByCode["帮助"]
	if !ok || helpLog.Status != 0 || helpLog.BotUID != botUID || helpLog.UserUID != 21001 {
		t.Fatalf("unexpected success log: %+v", helpLog)
	}
	alertLog, ok := logsByCode["警报"]
	if !ok || alertLog.Status != 1 || alertLog.ErrorMsg != "boom" {
		t.Fatalf("unexpected failure log: %+v", alertLog)
	}
}

const helpDocumentURLForTest = "https://kingprimes.top/posts/1bb16eb"

func echoPrivateMessage(t *testing.T, bot *zero.Ctx, botUID, userUID int64, text string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"post_type":    "message",
		"message_type": "private",
		"self_id":      botUID,
		"user_id":      userUID,
		"raw_message":  text,
		"message_id":   time.Now().UnixNano(),
		"message": []map[string]any{{
			"type": "text",
			"data": map[string]string{"text": text},
		}},
		"sender": map[string]any{"user_id": userUID, "nickname": "Tester"},
	})
	if err != nil {
		t.Fatal(err)
	}
	bot.Echo(payload)
}

// TestOneBotRuntimeConfig 验证运行模式和 client 必填地址校验，不启动 ZeroBot 全局运行时。
func TestOneBotRuntimeConfig(t *testing.T) {
	if _, err := onebot.NewRuntime(config.BotConfig{Mode: "invalid"}, botdirectory.NewDirectory()); err == nil {
		t.Fatal("invalid mode should fail")
	}
	if _, err := onebot.NewRuntime(config.BotConfig{Mode: config.BotModeClient}, botdirectory.NewDirectory()); err == nil {
		t.Fatal("client mode without URL should fail")
	}
	runtime, err := onebot.NewRuntime(config.BotConfig{Mode: config.BotModeServer, WsServerPath: "ws/custom"}, botdirectory.NewDirectory())
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	if err := runtime.RegisterRoutes(router); err != nil {
		t.Fatal(err)
	}
	if routes := router.Routes(); len(routes) != 1 || routes[0].Path != "/ws/custom" {
		t.Fatalf("unexpected registered routes: %+v", routes)
	}

	conflictingRouter := gin.New()
	conflictingRouter.GET("/ws/custom", func(*gin.Context) {})
	if err := runtime.RegisterRoutes(conflictingRouter); err == nil {
		t.Fatal("conflicting OneBot route should return an error instead of panicking")
	}
}
