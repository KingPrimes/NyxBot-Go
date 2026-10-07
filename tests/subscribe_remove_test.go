// 订阅删除接口测试：对齐 Java 拒绝语义（SubscriptionApplicationService）——
// 不存在报错；存在关联子项时拒绝删除（不做级联）；无关联才删除。
package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// seedSubscribe 造一个订阅组及可选的用户/检查类型，返回三层 ID（未造层返回 0）。
func seedSubscribe(t *testing.T, withUser, withType bool) (groupID, userID, typeID uint) {
	t.Helper()
	group := modelwarframe.MissionSubscribe{SubGroup: 10001, SubBotUID: 1, GroupName: "测试群"}
	if err := database.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if !withUser {
		return group.ID, 0, 0
	}
	user := modelwarframe.MissionSubscribeUser{SubID: group.ID, UserID: 20001, UserName: "测试用户"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if !withType {
		return group.ID, user.ID, 0
	}
	checkType := modelwarframe.MissionSubscribeUserCheckType{SubuID: user.ID, Subscribe: "ARBITRATION"}
	if err := database.DB.Create(&checkType).Error; err != nil {
		t.Fatal(err)
	}
	return group.ID, user.ID, checkType.ID
}

// decodeMsg 解析统一响应 { code, msg }。
func decodeMsg(t *testing.T, body []byte) (int, string) {
	t.Helper()
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response %q: %v", string(body), err)
	}
	return envelope.Code, envelope.Msg
}

// TestSubscribeRemoveRejectsGroupWithUsers 组内存在用户时拒绝删除且组保留（对齐 Java）。
func TestSubscribeRemoveRejectsGroupWithUsers(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	groupID, _, _ := seedSubscribe(t, true, true)

	rec := doRequest(t, r, http.MethodDelete, fmt.Sprintf("/data/warframe/subscribe/%d", groupID), token, "")
	code, msg := decodeMsg(t, rec.Body.Bytes())
	if code == 200 {
		t.Fatalf("存在关联用户时应拒绝删除，实际 code=%d msg=%q", code, msg)
	}
	if msg != "存在关联用户，无法删除订阅组" {
		t.Errorf("错误文案应对齐 Java，实际 %q", msg)
	}
	if got := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", groupID); got != 1 {
		t.Errorf("拒绝删除后订阅组应保留，实际剩余 %d", got)
	}
}

// TestSubscribeRemoveDeletesEmptyGroup 无关联用户的订阅组可正常删除。
func TestSubscribeRemoveDeletesEmptyGroup(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	groupID, _, _ := seedSubscribe(t, false, false)

	rec := doRequest(t, r, http.MethodDelete, fmt.Sprintf("/data/warframe/subscribe/%d", groupID), token, "")
	code, msg := decodeMsg(t, rec.Body.Bytes())
	if code != 200 {
		t.Fatalf("空订阅组应删除成功，实际 code=%d msg=%q", code, msg)
	}
	if got := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", groupID); got != 0 {
		t.Errorf("订阅组应已删除，实际剩余 %d", got)
	}
}

// TestSubscribeRemoveNotFound 订阅组不存在时报错（对齐 Java「订阅组不存在」）。
func TestSubscribeRemoveNotFound(t *testing.T) {
	r, token := setupWarframeDataRouter(t)

	rec := doRequest(t, r, http.MethodDelete, "/data/warframe/subscribe/99999", token, "")
	code, msg := decodeMsg(t, rec.Body.Bytes())
	if code == 200 || msg != "订阅组不存在" {
		t.Errorf("不存在时应报「订阅组不存在」，实际 code=%d msg=%q", code, msg)
	}
}

// TestSubscribeUserRemoveRejectsUserWithTypes 用户存在关联检查类型时拒绝删除。
func TestSubscribeUserRemoveRejectsUserWithTypes(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	_, userID, _ := seedSubscribe(t, true, true)

	rec := doRequest(t, r, http.MethodDelete, fmt.Sprintf("/data/warframe/subscribe/user/%d", userID), token, "")
	code, msg := decodeMsg(t, rec.Body.Bytes())
	if code == 200 {
		t.Fatalf("存在关联类型时应拒绝删除，实际 code=%d msg=%q", code, msg)
	}
	if msg != "存在关联类型，无法删除用户" {
		t.Errorf("错误文案应对齐 Java，实际 %q", msg)
	}
	if got := countRows(t, &modelwarframe.MissionSubscribeUser{}, "id = ?", userID); got != 1 {
		t.Errorf("拒绝删除后用户应保留，实际剩余 %d", got)
	}
}

// TestSubscribeUserRemoveDeletesEmptyUser 无关联类型的用户可正常删除。
func TestSubscribeUserRemoveDeletesEmptyUser(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	_, userID, _ := seedSubscribe(t, true, false)

	rec := doRequest(t, r, http.MethodDelete, fmt.Sprintf("/data/warframe/subscribe/user/%d", userID), token, "")
	code, msg := decodeMsg(t, rec.Body.Bytes())
	if code != 200 {
		t.Fatalf("无关联类型的用户应删除成功，实际 code=%d msg=%q", code, msg)
	}
	if got := countRows(t, &modelwarframe.MissionSubscribeUser{}, "id = ?", userID); got != 0 {
		t.Errorf("用户应已删除，实际剩余 %d", got)
	}
}

// TestSubscribeCheckTypeRemove 检查类型删除：存在删除成功、不存在报错。
func TestSubscribeCheckTypeRemove(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	_, _, typeID := seedSubscribe(t, true, true)

	// 不存在
	rec := doRequest(t, r, http.MethodDelete, "/data/warframe/subscribe/type/99999", token, "")
	if code, msg := decodeMsg(t, rec.Body.Bytes()); code == 200 || msg != "检查类型不存在" {
		t.Errorf("不存在时应报「检查类型不存在」，实际 code=%d msg=%q", code, msg)
	}

	// 存在：删除成功
	rec = doRequest(t, r, http.MethodDelete, fmt.Sprintf("/data/warframe/subscribe/type/%d", typeID), token, "")
	if code, msg := decodeMsg(t, rec.Body.Bytes()); code != 200 {
		t.Fatalf("检查类型应删除成功，实际 code=%d msg=%q", code, msg)
	}
	if got := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "id = ?", typeID); got != 0 {
		t.Errorf("检查类型应已删除，实际剩余 %d", got)
	}
}
