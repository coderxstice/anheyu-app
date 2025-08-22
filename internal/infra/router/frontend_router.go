package router

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	article_service "github.com/anzhiyu-c/anheyu-app/internal/app/service/article"
	"github.com/anzhiyu-c/anheyu-app/internal/app/service/setting"
	"github.com/anzhiyu-c/anheyu-app/internal/constant"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/parser"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/strutil"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

type CustomHTMLRender struct{ Templates *template.Template }

func (r CustomHTMLRender) Instance(name string, data interface{}) render.Render {
	return render.HTML{Template: r.Templates, Name: name, Data: data}
}

// SetupFrontend 封装了所有与前端静态资源和模板相关的配置
func SetupFrontend(engine *gin.Engine, settingSvc setting.SettingService, articleSvc *article_service.Service, embeddedFS embed.FS) {
	engine.GET("/manifest.json", func(c *gin.Context) {
		type ManifestIcon struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		}
		type WebAppManifest struct {
			Name            string         `json:"name"`
			ShortName       string         `json:"short_name"`
			Description     string         `json:"description"`
			ThemeColor      string         `json:"theme_color"`
			BackgroundColor string         `json:"background_color"`
			Display         string         `json:"display"`
			StartURL        string         `json:"start_url"`
			Icons           []ManifestIcon `json:"icons"`
		}

		manifest := WebAppManifest{
			Name:            settingSvc.Get(constant.KeyAppName.String()),
			ShortName:       settingSvc.Get(constant.KeyAppName.String()),
			Description:     settingSvc.Get(constant.KeySiteDescription.String()),
			ThemeColor:      settingSvc.Get(constant.KeyThemeColor.String()),
			BackgroundColor: "#ffffff",
			Display:         "standalone",
			StartURL:        "/",
			Icons: []ManifestIcon{
				{Src: settingSvc.Get(constant.KeyLogoURL192.String()), Sizes: "192x192", Type: "image/png"},
				{Src: settingSvc.Get(constant.KeyLogoURL512.String()), Sizes: "512x512", Type: "image/png"},
			},
		}
		if manifest.ThemeColor == "" {
			manifest.ThemeColor = "#ffffff"
		}
		c.JSON(http.StatusOK, manifest)
	})

	// 检查是否存在外部 'static' 目录用于覆盖
	overrideDir := "static"
	if _, err := os.Stat(overrideDir); !os.IsNotExist(err) {
		// --- 模式一: 外部 'static' 目录存在 ---
		engine.StaticFS("/", http.Dir(overrideDir))
		engine.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				response.Fail(c, http.StatusNotFound, "API 路由未找到")
				return
			}
			c.File(filepath.Join(overrideDir, "index.html"))
		})

	} else {
		// --- 模式二: 从嵌入的资源提供服务 ---
		distFS, err := fs.Sub(embeddedFS, "assets/dist")
		if err != nil {
			log.Fatalf("致命错误: 无法从嵌入的资源中创建 'assets/dist' 子文件系统: %v", err)
		}

		// 使用 template.FuncMap 来安全地序列化 JSON
		funcMap := template.FuncMap{
			"json": func(v interface{}) template.JS {
				a, _ := json.Marshal(v)
				return template.JS(a)
			},
		}
		parsedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFS(distFS, "index.html")
		if err != nil {
			log.Fatalf("解析嵌入式HTML模板失败: %v", err)
		}
		engine.HTMLRender = CustomHTMLRender{Templates: parsedTemplates}

		// 静态文件服务
		staticFS, _ := fs.Sub(distFS, "static")
		engine.StaticFS("/static", http.FS(staticFS))

		rootFiles, err := fs.ReadDir(distFS, ".")
		if err != nil {
			log.Fatalf("无法读取嵌入的 dist 根目录: %v", err)
		}

		for _, file := range rootFiles {
			if !file.IsDir() {
				fileName := file.Name()
				if fileName != "index.html" && !strings.HasSuffix(fileName, ".br") && !strings.HasSuffix(fileName, ".gz") {
					path := "/" + fileName
					engine.StaticFileFS(path, fileName, http.FS(distFS))
				}
			}
		}

		// --- SSR 核心处理器 ---
		engine.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				response.Fail(c, http.StatusNotFound, "API 路由未找到")
				return
			}

			// 检查是否是文章详情页的请求
			isPostDetail, _ := regexp.MatchString(`^/posts/([^/]+)$`, c.Request.URL.Path)
			if isPostDetail {
				slug := strings.TrimPrefix(c.Request.URL.Path, "/posts/")
				articleResponse, err := articleSvc.GetPublicBySlugOrID(c.Request.Context(), slug)

				if err == nil && articleResponse != nil {
					pageTitle := fmt.Sprintf("%s - %s", articleResponse.Title, settingSvc.Get(constant.KeyAppName.String()))

					var pageDescription string
					if len(articleResponse.Summaries) > 0 && articleResponse.Summaries[0] != "" {
						pageDescription = articleResponse.Summaries[0]
					} else {
						// 从文章HTML内容生成纯文本
						plainText := parser.StripHTML(articleResponse.ContentHTML)
						// 替换掉所有空白字符（换行、tab等），以便更准确地截断
						plainText = strings.Join(strings.Fields(plainText), " ")
						// 将纯文本截断到150个字符作为描述
						pageDescription = strutil.Truncate(plainText, 150)
					}
					// 如果自动生成的描述为空，则使用站点默认描述
					if pageDescription == "" {
						pageDescription = settingSvc.Get(constant.KeySiteDescription.String())
					}

					c.HTML(http.StatusOK, "index.html", gin.H{
						"siteName":        settingSvc.Get(constant.KeyAppName.String()),
						"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
						"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
						"siteScript":      settingSvc.Get(constant.KeyFooterCode.String()),
						"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
						"pageTitle":       pageTitle,
						"pageDescription": pageDescription,
						"themeColor":      articleResponse.PrimaryColor,
						"initialData":     articleResponse,
					})
					return
				}
			}

			// 对于其他所有路径或文章未找到的情况，使用站点默认元数据渲染
			defaultTitle := fmt.Sprintf("%s - %s", settingSvc.Get(constant.KeyAppName.String()), settingSvc.Get(constant.KeySubTitle.String()))
			c.HTML(http.StatusOK, "index.html", gin.H{
				"siteName":        settingSvc.Get(constant.KeyAppName.String()),
				"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
				"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
				"description":     settingSvc.Get(constant.KeySiteDescription.String()),
				"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
				"siteScript":      settingSvc.Get(constant.KeyFooterCode.String()),
				"pageTitle":       defaultTitle,
				"pageDescription": settingSvc.Get(constant.KeySiteDescription.String()),
				"themeColor":      "#406eeb",
				"initialData":     nil,
			})
		})
	}
}
