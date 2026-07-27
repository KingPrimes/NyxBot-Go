package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	botruntime "nyxbot-go/internal/bot"
	"nyxbot-go/internal/database"
	modelbot "nyxbot-go/internal/model/bot"
)

// setupBotConfigDB 初始化阶段 6 使用的五张 Bot 配置表，并清空进程级在线目录。
func setupBotConfigDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bot-config.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelbot.BotAdmin{},
		&modelbot.GroupWhite{},
		&modelbot.ProveWhite{},
		&modelbot.GroupBlack{},
		&modelbot.ProveBlack{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	botruntime.DefaultDirectory.Reset()
	t.Cleanup(func() {
		botruntime.DefaultDirectory.Reset()
		closeGlobalDB(t)
	})
}

type botEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func decodeBotEnvelope(t *testing.T, body []byte) botEnvelope {
	t.Helper()
	var envelope botEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response %q: %v", string(body), err)
	}
	return envelope
}

func decodeOptions(t *testing.T, body []byte) (int, []botruntime.Option) {
	t.Helper()
	envelope := decodeBotEnvelope(t, body)
	var options []botruntime.Option
	if envelope.Code == http.StatusOK {
		if err := json.Unmarshal(envelope.Data, &options); err != nil {
			t.Fatalf("decode options: %v", err)
		}
	}
	return envelope.Code, options
}

// TestBotDirectoryEndpoints 验证在线 Bot、好友和群组选项以及离线错误语义。
func TestBotDirectoryEndpoints(t *testing.T) {
	setupBotConfigDB(t)
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	if rec := doRequest(t, r, http.MethodGet, "/config/bot/bots", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bots without token: expected 401, got %d", rec.Code)
	}
	if rec := doRequest(t, r, http.MethodGet, "/config/bot/bots", token, ""); rec.Code != http.StatusInternalServerError {
		t.Fatalf("empty directory: expected 500, got %d, body: %s", rec.Code, rec.Body.String())
	}

	botruntime.DefaultDirectory.UpsertBot(20002, "Nyx-2")
	botruntime.DefaultDirectory.UpsertBot(10001, "Nyx-1")
	botruntime.DefaultDirectory.SetFriends(10001, []botruntime.Option{{Label: "Alice", Value: "30003"}})
	botruntime.DefaultDirectory.SetGroups(10001, []botruntime.Option{{Label: "Tenno", Value: "40004"}})

	code, bots := decodeOptions(t, doRequest(t, r, http.MethodGet, "/config/bot/bots", token, "").Body.Bytes())
	if code != 200 || len(bots) != 2 || bots[0].Value != "10001" || bots[1].Value != "20002" {
		t.Fatalf("bot options mismatch: code=%d options=%+v", code, bots)
	}
	_, friends := decodeOptions(t, doRequest(t, r, http.MethodGet, "/config/bot/friend/10001", token, "").Body.Bytes())
	if len(friends) != 1 || friends[0].Label != "Alice" {
		t.Fatalf("friend options mismatch: %+v", friends)
	}
	_, groups := decodeOptions(t, doRequest(t, r, http.MethodGet, "/config/bot/group/10001", token, "").Body.Bytes())
	if len(groups) != 1 || groups[0].Value != "40004" {
		t.Fatalf("group options mismatch: %+v", groups)
	}

	recorder := doRequest(t, r, http.MethodGet, "/config/bot/friend/99999", token, "")
	if recorder.Code != http.StatusOK || decodeBotEnvelope(t, recorder.Body.Bytes()).Code != http.StatusNoContent {
		t.Fatalf("offline bot should use HTTP 200 with business code 204, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// TestBotAdminCRUD 验证权限选项、数字字符串兼容、超级管理员唯一性和管理员增删查。
func TestBotAdminCRUD(t *testing.T) {
	setupBotConfigDB(t)
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	code, permissions := decodeOptions(t, doRequest(t, r, http.MethodGet, "/config/bot/admin/permissions", token, "").Body.Bytes())
	if code != 200 || len(permissions) != 3 || permissions[0].Value != "SUPER_ADMIN" || permissions[2].Value != "USER" {
		t.Fatalf("permissions mismatch: code=%d options=%+v", code, permissions)
	}

	save := func(body string) *botEnvelope {
		t.Helper()
		recorder := doRequest(t, r, http.MethodPost, "/config/bot/admin/save", token, body)
		envelope := decodeBotEnvelope(t, recorder.Body.Bytes())
		return &envelope
	}
	if got := save(`{"botUid":"10001","adminUid":"20001","permissions":"SUPER_ADMIN"}`); got.Code != 200 {
		t.Fatalf("save super admin failed: %+v", got)
	}
	if got := save(`{"botUid":10001,"adminUid":20002,"permissions":"SUPER_ADMIN"}`); got.Code != 500 {
		t.Fatalf("duplicate super admin should fail: %+v", got)
	}
	if got := save(`{"botUid":10002,"adminUid":20002,"permissions":"SUPER_ADMIN"}`); got.Code != 200 {
		t.Fatalf("another bot may have a super admin: %+v", got)
	}
	if got := save(`{"botUid":10001,"adminUid":20003,"permissions":"MANAGE"}`); got.Code != 500 {
		t.Fatalf("MANAGE permission should be rejected: %+v", got)
	}
	if got := save(`{"botUid":1,"adminUid":2,"permissions":"ADMIN"}`); got.Code != 400 {
		t.Fatalf("invalid QQ should be rejected: %+v", got)
	}

	_, data := decodeData(t, doRequest(t, r, http.MethodPost, "/config/bot/admin/list", token, `{"current":1,"size":15,"botUid":"10001"}`))
	if data["total"] != float64(1) {
		t.Fatalf("filtered admin total mismatch: %v", data)
	}
	records := data["records"].([]any)
	row := records[0].(map[string]any)
	if row["botUid"] != float64(10001) || row["adminUid"] != float64(20001) || row["permissions"] != "SUPER_ADMIN" {
		t.Fatalf("admin JSON contract mismatch: %v", row)
	}

	id := int(row["id"].(float64))
	recorder := doRequest(t, r, http.MethodDelete, "/config/bot/admin/remove/"+jsonNumber(id), token, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("remove admin failed: %s", recorder.Body.String())
	}
}

// TestBotWhiteAndBlackLists 验证四类名单分页、全局业务键更新、冲突校验和删除。
func TestBotWhiteAndBlackLists(t *testing.T) {
	setupBotConfigDB(t)
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	for _, body := range []string{
		`{"botUid":"10001","groupUid":"30001"}`,
		`{"botUid":10001,"groupUid":30002}`,
		`{"botUid":10001,"groupUid":30003}`,
	} {
		assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/black/group/save", token, body), 200)
	}
	_, data := decodeData(t, doRequest(t, r, http.MethodPost, "/config/bot/black/group/list", token, `{"current":2,"size":2}`))
	if data["total"] != float64(3) || data["current"] != float64(2) || len(data["records"].([]any)) != 1 {
		t.Fatalf("black group pagination mismatch: %v", data)
	}

	assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/white/group/save", token,
		`{"botUid":10001,"groupUid":30001}`), 400)
	assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/white/group/save", token,
		`{"botUid":"10001","groupUid":"40001"}`), 200)
	// Java 服务按 groupUid 全局 upsert，同一群号会更新 Bot 而不是新增第二条。
	assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/white/group/save", token,
		`{"botUid":"10002","groupUid":"40001"}`), 200)
	_, data = decodeData(t, doRequest(t, r, http.MethodPost, "/config/bot/white/group/list", token,
		`{"current":1,"size":15,"groupUid":"40001"}`))
	whiteGroupRows := data["records"].([]any)
	if data["total"] != float64(1) || whiteGroupRows[0].(map[string]any)["botUid"] != float64(10002) {
		t.Fatalf("white group upsert mismatch: %v", data)
	}

	assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/black/prove/save", token,
		`{"botUid":10001,"proveUid":50001}`), 200)
	assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/white/prove/save", token,
		`{"botUid":10001,"proveUid":50001}`), 400)
	assertBusinessCode(t, doRequest(t, r, http.MethodPost, "/config/bot/white/prove/save", token,
		`{"botUid":"10001","proveUid":"60001"}`), 200)

	_, data = decodeData(t, doRequest(t, r, http.MethodPost, "/config/bot/black/prove/list", token,
		`{"current":1,"size":15,"proveUid":"50001"}`))
	blackProveRow := data["records"].([]any)[0].(map[string]any)
	_, whiteProveData := decodeData(t, doRequest(t, r, http.MethodPost, "/config/bot/white/prove/list", token,
		`{"current":1,"size":15,"proveUid":60001}`))
	whiteProveRow := whiteProveData["records"].([]any)[0].(map[string]any)

	removeCases := []struct {
		path string
		id   int
	}{
		{"/config/bot/black/group/remove/", int(dataID(t, "/config/bot/black/group/list", r, token, `{"current":1,"size":15,"groupUid":30001}`))},
		{"/config/bot/white/group/remove/", int(whiteGroupRows[0].(map[string]any)["id"].(float64))},
		{"/config/bot/black/prove/remove/", int(blackProveRow["id"].(float64))},
		{"/config/bot/white/prove/remove/", int(whiteProveRow["id"].(float64))},
	}
	for _, testCase := range removeCases {
		assertBusinessCode(t, doRequest(t, r, http.MethodDelete, testCase.path+jsonNumber(testCase.id), token, ""), 200)
	}
}

func assertBusinessCode(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()
	if got := decodeBotEnvelope(t, recorder.Body.Bytes()).Code; got != want {
		t.Fatalf("expected business code %d, got %d", want, got)
	}
}

func dataID(t *testing.T, path string, router *gin.Engine, token, body string) float64 {
	t.Helper()
	recorder := doRequest(t, router, http.MethodPost, path, token, body)
	_, data := decodeData(t, recorder)
	return data["records"].([]any)[0].(map[string]any)["id"].(float64)
}

func jsonNumber(value int) string {
	return strconv.Itoa(value)
}
