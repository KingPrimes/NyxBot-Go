// 测试包入口：把未翻译清单的输出重定向到临时目录。
// 世界状态/节点翻译未命中会在运行期写未翻译清单（internal/warframe.RecordUntranslated），
// 不重定向就会把文件写进 tests/data 污染工作目录。
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"nyxbot-go/internal/warframe"
)

// TestMain 在全部用例执行前重定向未翻译清单路径，跑完清理临时目录并还原原路径。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nyxbot-untranslated-")
	if err != nil {
		panic(err)
	}
	previous := warframe.SetUntranslatedPath(filepath.Join(dir, "UntranslatedRelicsRewardsName.json"))
	code := m.Run()
	_ = os.RemoveAll(dir)
	warframe.SetUntranslatedPath(previous)
	os.Exit(code)
}
