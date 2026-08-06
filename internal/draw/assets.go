// 绘图资源嵌入：思源宋体粗体、Warframe 图标字体、看板娘插画
// 来源：draw-image-plugin-core/src/main/resources（Java 侧原样拷贝，go:embed 随二进制分发）
package draw

import (
	"bytes"
	"embed"
	"image"
	"image/png"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

// assetsFS 嵌入内部绘图资源目录。
//
//go:embed assets
var assetsFS embed.FS

// fontFile 按文件名读取嵌入字体文件。
func fontFile(name string) ([]byte, error) {
	return fs.ReadFile(assetsFS, "assets/fonts/"+name)
}

// standingCache 看板娘图片缓存（懒加载，空切片表示无资源）。
var (
	standingOnce  sync.Once
	standingCache []image.Image
)

// standingImages 加载全部看板娘图片（按文件名数字排序，对齐 Java ImageIOUtils）。
func standingImages() []image.Image {
	standingOnce.Do(func() {
		entries, err := fs.ReadDir(assetsFS, "assets/image")
		if err != nil {
			return
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".png") {
				continue
			}
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			raw, readErr := fs.ReadFile(assetsFS, "assets/image/"+name)
			if readErr != nil {
				continue
			}
			img, decodeErr := png.Decode(bytes.NewReader(raw))
			if decodeErr == nil {
				standingCache = append(standingCache, img)
			}
		}
	})
	return standingCache
}
