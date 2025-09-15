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
	"time"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/parser"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/strutil"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	article_service "github.com/anzhiyu-c/anheyu-app/pkg/service/article"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

type CustomHTMLRender struct{ Templates *template.Template }

func (r CustomHTMLRender) Instance(name string, data interface{}) render.Render {
	return render.HTML{Template: r.Templates, Name: name, Data: data}
}

// getRequestScheme 确定请求的协议 (http 或 https)
func getRequestScheme(c *gin.Context) string {
	// 优先检查 X-Forwarded-Proto Header，这在反向代理后很常见
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	// 检查请求的 TLS 字段
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

// SetupFrontend 封装了所有与前端静态资源和模板相关的配置
func SetupFrontend(engine *gin.Engine, settingSvc setting.SettingService, articleSvc article_service.Service, embeddedFS embed.FS) {
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

	// 准备一个通用的模板函数映射
	funcMap := template.FuncMap{
		"json": func(v interface{}) template.JS {
			a, _ := json.Marshal(v)
			return template.JS(a)
		},
	}

	overrideDir := "static"
	if _, err := os.Stat(overrideDir); !os.IsNotExist(err) {
		// --- 模式一: 外部 'static' 目录存在 (支持SSR) ---
		log.Println("正在使用 [模式一]: 外部 'static' 目录.")

		parsedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFiles(filepath.Join(overrideDir, "index.html"))
		if err != nil {
			log.Fatalf("解析外部HTML模板失败: %v", err)
		}
		engine.HTMLRender = CustomHTMLRender{Templates: parsedTemplates}

		engine.StaticFS("/static", http.Dir(filepath.Join(overrideDir, "static")))
		rootFiles, err := os.ReadDir(overrideDir)
		if err != nil {
			log.Fatalf("无法读取外部 'static' 目录: %v", err)
		}
		for _, file := range rootFiles {
			if !file.IsDir() {
				fileName := file.Name()
				if fileName != "index.html" {
					engine.StaticFile("/"+fileName, filepath.Join(overrideDir, fileName))
				}
			}
		}

		engine.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				response.Fail(c, http.StatusNotFound, "API 路由未找到")
				return
			}

			// 获取完整的当前页面 URL
			fullURL := fmt.Sprintf("%s://%s%s", getRequestScheme(c), c.Request.Host, c.Request.URL.RequestURI())

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
						plainText := parser.StripHTML(articleResponse.ContentHTML)
						plainText = strings.Join(strings.Fields(plainText), " ")
						pageDescription = strutil.Truncate(plainText, 150)
					}
					if pageDescription == "" {
						pageDescription = settingSvc.Get(constant.KeySiteDescription.String())
					}

					// 构建文章标签列表
					articleTags := make([]string, len(articleResponse.PostTags))
					for i, tag := range articleResponse.PostTags {
						articleTags[i] = tag.Name
					}

					c.HTML(http.StatusOK, "index.html", gin.H{
						// --- 基础 SEO 和页面信息 ---
						"pageTitle":       pageTitle,
						"pageDescription": pageDescription,
						"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
						"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
						"themeColor":      articleResponse.PrimaryColor,
						"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
						// --- 用于 Vue 水合的数据 ---
						"initialData":   articleResponse,
						"ogType":        "article",
						"ogUrl":         fullURL,
						"ogTitle":       pageTitle,
						"ogDescription": pageDescription,
						"ogImage":       articleResponse.CoverURL,
						"ogSiteName":    settingSvc.Get(constant.KeyAppName.String()),
						"ogLocale":      "zh_CN",
						// --- Article 元标签数据 ---
						"articlePublishedTime": articleResponse.CreatedAt.Format(time.RFC3339),
						"articleModifiedTime":  articleResponse.UpdatedAt.Format(time.RFC3339),
						"articleAuthor":        articleResponse.CopyrightAuthor,
						"articleTags":          articleTags,
						// --- 网站脚本和额外信息 ---
						"siteScript": template.HTML(settingSvc.Get(constant.KeyFooterCode.String())),
					})
					return
				}
			}
			// --- 默认页面渲染 ---
			defaultTitle := fmt.Sprintf("%s - %s", settingSvc.Get(constant.KeyAppName.String()), settingSvc.Get(constant.KeySubTitle.String()))
			defaultDescription := settingSvc.Get(constant.KeySiteDescription.String())
			defaultImage := settingSvc.Get(constant.KeyLogoURL512.String())

			c.HTML(http.StatusOK, "index.html", gin.H{
				// --- 基础 SEO 和页面信息 ---
				"pageTitle":       defaultTitle,
				"pageDescription": defaultDescription,
				"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
				"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
				"themeColor":      "#f7f9fe",
				"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
				// --- 用于 Vue 水合的数据 ---
				"initialData":   nil,
				"ogType":        "website",
				"ogUrl":         fullURL,
				"ogTitle":       defaultTitle,
				"ogDescription": defaultDescription,
				"ogImage":       defaultImage,
				"ogSiteName":    settingSvc.Get(constant.KeyAppName.String()),
				"ogLocale":      "zh_CN",
				// --- Article 元标签数据 (默认为空) ---
				"articlePublishedTime": nil,
				"articleModifiedTime":  nil,
				"articleAuthor":        nil,
				"articleTags":          nil,
				// --- 网站脚本和额外信息 ---
				"siteScript": template.HTML(settingSvc.Get(constant.KeyFooterCode.String())),
			})
		})

	} else {
		// --- 模式二: 从嵌入的资源提供服务 ---
		log.Println("正在使用 [模式二]: 嵌入式静态资源.")

		distFS, err := fs.Sub(embeddedFS, "assets/dist")
		if err != nil {
			log.Fatalf("致命错误: 无法从嵌入的资源中创建 'assets/dist' 子文件系统: %v", err)
		}

		parsedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFS(distFS, "index.html")
		if err != nil {
			log.Fatalf("解析嵌入式HTML模板失败: %v", err)
		}
		engine.HTMLRender = CustomHTMLRender{Templates: parsedTemplates}

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

		engine.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				response.Fail(c, http.StatusNotFound, "API 路由未找到")
				return
			}
			// 获取完整的当前页面 URL
			fullURL := fmt.Sprintf("%s://%s%s", getRequestScheme(c), c.Request.Host, c.Request.URL.RequestURI())

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
						plainText := parser.StripHTML(articleResponse.ContentHTML)
						plainText = strings.Join(strings.Fields(plainText), " ")
						pageDescription = strutil.Truncate(plainText, 150)
					}
					if pageDescription == "" {
						pageDescription = settingSvc.Get(constant.KeySiteDescription.String())
					}

					// 构建文章标签列表
					articleTags := make([]string, len(articleResponse.PostTags))
					for i, tag := range articleResponse.PostTags {
						articleTags[i] = tag.Name
					}

					c.HTML(http.StatusOK, "index.html", gin.H{
						// --- 基础 SEO 和页面信息 ---
						"pageTitle":       pageTitle,
						"pageDescription": pageDescription,
						"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
						"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
						"themeColor":      articleResponse.PrimaryColor,
						"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
						// --- 用于 Vue 水合的数据 ---
						"initialData":   articleResponse,
						"ogType":        "article",
						"ogUrl":         fullURL,
						"ogTitle":       pageTitle,
						"ogDescription": pageDescription,
						"ogImage":       articleResponse.CoverURL,
						"ogSiteName":    settingSvc.Get(constant.KeyAppName.String()),
						"ogLocale":      "zh_CN",
						// --- Article 元标签数据 ---
						"articlePublishedTime": articleResponse.CreatedAt.Format(time.RFC3339),
						"articleModifiedTime":  articleResponse.UpdatedAt.Format(time.RFC3339),
						"articleAuthor":        articleResponse.CopyrightAuthor,
						"articleTags":          articleTags,
						// --- 网站脚本和额外信息 ---
						"siteScript": template.HTML(settingSvc.Get(constant.KeyFooterCode.String())),
					})
					return
				}
			}

			// --- 默认页面渲染 ---
			defaultTitle := fmt.Sprintf("%s - %s", settingSvc.Get(constant.KeyAppName.String()), settingSvc.Get(constant.KeySubTitle.String()))
			defaultDescription := settingSvc.Get(constant.KeySiteDescription.String())
			defaultImage := settingSvc.Get(constant.KeyLogoURL512.String())

			c.HTML(http.StatusOK, "index.html", gin.H{
				// --- 基础 SEO 和页面信息 ---
				"pageTitle":       defaultTitle,
				"pageDescription": defaultDescription,
				"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
				"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
				"themeColor":      "#f7f9fe",
				"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
				// --- 用于 Vue 水合的数据 ---
				"initialData":   nil,
				"ogType":        "website",
				"ogUrl":         fullURL,
				"ogTitle":       defaultTitle,
				"ogDescription": defaultDescription,
				"ogImage":       defaultImage,
				"ogSiteName":    settingSvc.Get(constant.KeyAppName.String()),
				"ogLocale":      "zh_CN",
				// --- Article 元标签数据 (默认为空) ---
				"articlePublishedTime": nil,
				"articleModifiedTime":  nil,
				"articleAuthor":        nil,
				"articleTags":          nil,
				// --- 网站脚本和额外信息 ---
				"siteScript": template.HTML(settingSvc.Get(constant.KeyFooterCode.String())),
			})
		})
	}
}
