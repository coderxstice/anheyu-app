// anheyu-app/pkg/service/utility/color.go
package utility

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"

	"github.com/EdlinOrg/prominentcolor"
	_ "golang.org/x/image/webp"
)

type ColorService struct{}

func NewColorService() *ColorService {
	log.Println("[ColorService] 初始化颜色服务：使用 'prominentcolor' (K-Means算法) 来查找主色调。")
	return &ColorService{}
}

func (s *ColorService) GetPrimaryColor(reader io.Reader) (string, error) {
	imgData, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", fmt.Errorf("解码图片失败: %w", err)
	}

	colors, err := prominentcolor.KmeansWithArgs(
		1,
		img,
	)
	if err != nil {
		return "", fmt.Errorf("使用 prominentcolor (K-Means) 提取主色调失败: %w", err)
	}

	if len(colors) == 0 {
		return "", fmt.Errorf("prominentcolor (K-Means) 未能找到任何主色调")
	}

	dominantColor := colors[0].Color

	return fmt.Sprintf("#%02x%02x%02x", dominantColor.R, dominantColor.G, dominantColor.B), nil
}
