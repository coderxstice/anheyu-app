/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-15 11:30:55
 * @LastEditTime: 2025-09-01 23:34:24
 * @LastEditors: 安知鱼
 */
// anheyu-app/pkg/router/router.go
package router

import (
	"github.com/gin-gonic/gin"

	"github.com/anzhiyu-c/anheyu-app/internal/app/middleware"
	album_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/album"
	article_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/article"
	auth_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/auth"
	comment_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/comment"
	direct_link_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/direct_link"
	file_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/file"
	link_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/link"
	page_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/page"
	post_category_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/post_category"
	post_tag_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/post_tag"
	proxy_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/proxy"
	public_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/public"
	search_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/search"
	setting_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/setting"
	statistics_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/statistics"
	storage_policy_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/storage_policy"
	thumbnail_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/thumbnail"
	user_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/user"
)

// Router 封装了应用的所有路由和其依赖的处理器。
type Router struct {
	authHandler          *auth_handler.AuthHandler
	albumHandler         *album_handler.AlbumHandler
	userHandler          *user_handler.UserHandler
	publicHandler        *public_handler.PublicHandler
	settingHandler       *setting_handler.SettingHandler
	storagePolicyHandler *storage_policy_handler.StoragePolicyHandler
	fileHandler          *file_handler.FileHandler
	directLinkHandler    *direct_link_handler.DirectLinkHandler
	thumbnailHandler     *thumbnail_handler.ThumbnailHandler
	articleHandler       *article_handler.Handler
	postTagHandler       *post_tag_handler.Handler
	postCategoryHandler  *post_category_handler.Handler
	commentHandler       *comment_handler.Handler
	linkHandler          *link_handler.Handler
	pageHandler          *page_handler.Handler
	statisticsHandler    *statistics_handler.StatisticsHandler
	mw                   *middleware.Middleware
	searchHandler        *search_handler.Handler
	proxyHandler         *proxy_handler.ProxyHandler
}

// NewRouter 是 Router 的构造函数，通过依赖注入接收所有处理器。
func NewRouter(
	authHandler *auth_handler.AuthHandler,
	albumHandler *album_handler.AlbumHandler,
	userHandler *user_handler.UserHandler,
	publicHandler *public_handler.PublicHandler,
	settingHandler *setting_handler.SettingHandler,
	storagePolicyHandler *storage_policy_handler.StoragePolicyHandler,
	fileHandler *file_handler.FileHandler,
	directLinkHandler *direct_link_handler.DirectLinkHandler,
	thumbnailHandler *thumbnail_handler.ThumbnailHandler,
	articleHandler *article_handler.Handler,
	postTagHandler *post_tag_handler.Handler,
	postCategoryHandler *post_category_handler.Handler,
	commentHandler *comment_handler.Handler,
	linkHandler *link_handler.Handler,
	pageHandler *page_handler.Handler,
	statisticsHandler *statistics_handler.StatisticsHandler,
	mw *middleware.Middleware,
	searchHandler *search_handler.Handler,
	proxyHandler *proxy_handler.ProxyHandler,
) *Router {
	return &Router{
		authHandler:          authHandler,
		albumHandler:         albumHandler,
		userHandler:          userHandler,
		publicHandler:        publicHandler,
		settingHandler:       settingHandler,
		storagePolicyHandler: storagePolicyHandler,
		fileHandler:          fileHandler,
		directLinkHandler:    directLinkHandler,
		thumbnailHandler:     thumbnailHandler,
		articleHandler:       articleHandler,
		postTagHandler:       postTagHandler,
		postCategoryHandler:  postCategoryHandler,
		commentHandler:       commentHandler,
		linkHandler:          linkHandler,
		pageHandler:          pageHandler,
		statisticsHandler:    statisticsHandler,
		mw:                   mw,
		searchHandler:        searchHandler,
		proxyHandler:         proxyHandler,
	}
}

// Setup 将所有路由注册到 Gin 引擎。
// 这是在 main.go 中将被调用的唯一入口点。
func (r *Router) Setup(engine *gin.Engine) {
	// 创建 /api 分组
	apiGroup := engine.Group("/api")

	// 文件下载
	apiGroup.GET("/f/:publicID/*filename", r.directLinkHandler.HandleDirectDownload)

	// 获取缩略图
	apiGroup.GET("/t/:signedToken", r.thumbnailHandler.HandleThumbnailContent)

	// 需要被缓存的路由不在/api 下
	downloadGroup := engine.Group("/needcache")
	{
		downloadGroup.GET("/download/:public_id", r.fileHandler.HandleUniversalSignedDownload)
	}

	// 代理路由
	apiGroup.GET("/proxy/download", r.proxyHandler.HandleDownload)

	// 注册各个模块的路由
	r.registerAuthRoutes(apiGroup)
	r.registerAlbumRoutes(apiGroup)
	r.registerUserRoutes(apiGroup)
	r.registerPublicRoutes(apiGroup)
	r.registerSettingRoutes(apiGroup)
	r.registerStoragePolicyRoutes(apiGroup)
	r.registerFileRoutes(apiGroup)
	r.registerDirectLinkRoutes(apiGroup)
	r.registerThumbnailRoutes(apiGroup)
	r.registerArticleRoutes(apiGroup)
	r.registerPostTagRoutes(apiGroup)
	r.registerPostCategoryRoutes(apiGroup)
	r.registerCommentRoutes(apiGroup)
	r.registerPageRoutes(apiGroup)
	r.registerSearchRoutes(apiGroup)
	r.registerLinkRoutes(apiGroup)
	r.registerStatisticsRoutes(apiGroup)
}

func (r *Router) registerCommentRoutes(api *gin.RouterGroup) {
	// 公开的评论接口
	commentsPublic := api.Group("/public/comments")
	{
		commentsPublic.GET("", r.commentHandler.ListByPath)

		commentsPublic.GET("/latest", r.commentHandler.ListLatest)

		commentsPublic.GET("/:id/children", r.commentHandler.ListChildren)

		commentsPublic.POST("", r.mw.JWTAuthOptional(), r.commentHandler.Create)
		commentsPublic.POST("/upload", r.mw.JWTAuthOptional(), r.commentHandler.UploadCommentImage)
		commentsPublic.POST("/:id/like", r.commentHandler.LikeComment)
		commentsPublic.POST("/:id/unlike", r.commentHandler.UnlikeComment)
	}

	// 管理员专属的评论接口
	commentsAdmin := api.Group("/comments").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		commentsAdmin.GET("", r.commentHandler.AdminList)
		commentsAdmin.DELETE("", r.commentHandler.Delete)
		commentsAdmin.PUT("/:id/status", r.commentHandler.UpdateStatus)
		commentsAdmin.PUT("/:id/pin", r.commentHandler.SetPin)
	}
}

func (r *Router) registerPostTagRoutes(api *gin.RouterGroup) {
	// 列表查询通常是公开的，或只需登录
	postTagsPublic := api.Group("/post-tags")
	{
		postTagsPublic.GET("", r.postTagHandler.List)
		// postTagsPublic.GET("/:id", r.postTagHandler.Get)
	}

	// 创建、更新、删除通常需要管理员权限
	postTagsAdmin := api.Group("/post-tags").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		postTagsAdmin.POST("", r.postTagHandler.Create)
		postTagsAdmin.PUT("/:id", r.postTagHandler.Update)
		postTagsAdmin.DELETE("/:id", r.postTagHandler.Delete)
	}
}

func (r *Router) registerPostCategoryRoutes(api *gin.RouterGroup) {
	postCategoriesPublic := api.Group("/post-categories")
	{
		postCategoriesPublic.GET("", r.postCategoryHandler.List)
		// postCategoriesPublic.GET("/:id", r.postCategoryHandler.Get)
	}

	postCategoriesAdmin := api.Group("/post-categories").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		postCategoriesAdmin.POST("", r.postCategoryHandler.Create)
		postCategoriesAdmin.PUT("/:id", r.postCategoryHandler.Update)
		postCategoriesAdmin.DELETE("/:id", r.postCategoryHandler.Delete)
	}
}

func (r *Router) registerArticleRoutes(api *gin.RouterGroup) {
	// 后台管理接口，需要认证和管理员权限
	articlesAdmin := api.Group("/articles").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		articlesAdmin.POST("", r.articleHandler.Create)
		articlesAdmin.GET("", r.articleHandler.List)
		articlesAdmin.GET("/:id", r.articleHandler.Get)
		articlesAdmin.PUT("/:id", r.articleHandler.Update)
		articlesAdmin.DELETE("/:id", r.articleHandler.Delete)
		articlesAdmin.POST("/upload", r.articleHandler.UploadImage)
	}

	articlesPublic := api.Group("/public/articles")
	{
		articlesPublic.GET("", r.articleHandler.ListPublic)
		articlesPublic.GET("/home", r.articleHandler.ListHome)
		articlesPublic.GET("/random", r.articleHandler.GetRandom)
		articlesPublic.GET("/archives", r.articleHandler.ListArchives)
		// 注意：把带参数的路由放在最后，避免路由冲突
		articlesPublic.GET("/:id", r.articleHandler.GetPublic)
	}
}

func (r *Router) registerThumbnailRoutes(api *gin.RouterGroup) {
	// 预览/缩略图的获取需要登录，以保护私有文件
	thumbnail := api.Group("/thumbnail").Use(r.mw.JWTAuth())
	{

		// 手动重新生成缩略图的接口
		// POST /api/thumbnail/regenerate
		thumbnail.POST("/regenerate", r.thumbnailHandler.RegenerateThumbnail)

		// POST /api/thumbnail/regenerate/directory
		thumbnail.POST("/regenerate/directory", r.thumbnailHandler.RegenerateThumbnailsForDirectory)

		thumbnail.GET("/:publicID", r.thumbnailHandler.GetThumbnailSign)
	}
}

// registerAuthRoutes 注册认证相关的路由
func (r *Router) registerAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	{
		auth.POST("/login", r.authHandler.Login)
		auth.POST("/register", r.authHandler.Register)
		auth.POST("/refresh-token", r.authHandler.RefreshToken)
		auth.POST("/activate", r.authHandler.ActivateUser)
		auth.POST("/forgot-password", r.authHandler.ForgotPasswordRequest)
		auth.POST("/reset-password", r.authHandler.ResetPassword)
		auth.GET("/check-email", r.authHandler.CheckEmail)
	}
}

// registerAlbumRoutes 注册相册相关的路由 (后台管理)
func (r *Router) registerAlbumRoutes(api *gin.RouterGroup) {
	albums := api.Group("/albums").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		albums.GET("/get", r.albumHandler.GetAlbums)
		albums.POST("/add", r.albumHandler.AddAlbum)
		albums.PUT("/update/:id", r.albumHandler.UpdateAlbum)
		albums.DELETE("/delete/:id", r.albumHandler.DeleteAlbum)
	}
}

// registerSettingRoutes 注册站点配置相关的路由
func (r *Router) registerSettingRoutes(api *gin.RouterGroup) {
	settings := api.Group("/settings").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		settings.POST("/get-by-keys", r.settingHandler.GetSettingsByKeys)
		settings.POST("/update", r.settingHandler.UpdateSettings)
		settings.POST("/test-email", r.settingHandler.TestEmail)
	}
}

// registerUserRoutes 注册用户相关的路由
func (r *Router) registerUserRoutes(api *gin.RouterGroup) {
	user := api.Group("/user").Use(r.mw.JWTAuth())
	{
		user.GET("/info", r.userHandler.GetUserInfo)
		user.POST("/update-password", r.userHandler.UpdateUserPassword)
	}
}

// registerPublicRoutes 注册公开的、无需认证的路由
func (r *Router) registerPublicRoutes(api *gin.RouterGroup) {
	public := api.Group("/public")
	{
		public.GET("/albums", r.publicHandler.GetPublicAlbums)
		public.PUT("/stat/:id", r.publicHandler.UpdateAlbumStat)
		public.GET("/site-config", r.settingHandler.GetSiteConfig)
	}
}

// registerStoragePolicyRoutes 注册存储策略相关的路由
func (r *Router) registerStoragePolicyRoutes(api *gin.RouterGroup) {
	policies := api.Group("/policies").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		policies.POST("", r.storagePolicyHandler.Create)
		policies.GET("", r.storagePolicyHandler.List)
		policies.GET("/connect/onedrive/:id", r.storagePolicyHandler.ConnectOneDrive)
		policies.POST("/authorize/onedrive", r.storagePolicyHandler.AuthorizeOneDrive)
		policies.GET("/:id", r.storagePolicyHandler.Get)
		policies.PUT("/:id", r.storagePolicyHandler.Update)
		policies.DELETE("/:id", r.storagePolicyHandler.Delete)
	}
}

// registerFileRoutes 注册文件相关的路由
func (r *Router) registerFileRoutes(api *gin.RouterGroup) {
	// --- 文件浏览路由 ---
	// GET /api/files?uri=...
	// 注意：这里只应用JWTAuth()。因为GetFilesByPath处理器内部已经包含了区分
	filesGroup := api.Group("/file")

	// 获取文件内容
	filesGroup.GET("/content", r.fileHandler.ServeSignedContent)

	filesGroup.Use(r.mw.JWTAuth())
	{
		// 获取文件列表
		filesGroup.GET("", r.fileHandler.GetFilesByPath)
		filesGroup.GET("/:id", r.fileHandler.GetFileInfo)
		filesGroup.GET("/download/:id", r.fileHandler.DownloadFile)

		// POST /api/file/create
		filesGroup.POST("/create", r.fileHandler.CreateEmptyFile)
		filesGroup.PUT("/content/:publicID", r.fileHandler.UpdateFileContentByID)
		// Delete /api/file/?ids=...
		filesGroup.DELETE("", r.fileHandler.DeleteItems)
		// PUT /api/file/rename
		filesGroup.PUT("/rename", r.fileHandler.RenameItem)

		// 获取文件夹的预览图像URL
		// 这个接口用于获取文件夹内所有图片的预览图像URL
		filesGroup.GET("/preview-urls", r.fileHandler.GetPreviewURLs)
	}

	// --- 文件上传路由 ---
	// 上传相关操作也只需要JWT认证，具体权限由Handler处理
	uploadGroup := filesGroup.Group("/upload")
	uploadGroup.Use(r.mw.JWTAuth())
	{
		// 创建上传会话
		// PUT /api/file/upload
		uploadGroup.PUT("", r.fileHandler.CreateUploadSession)

		// 获取上传会话状态
		// GET /api/file/upload/session/{sessionId}
		uploadGroup.GET("/session/:sessionId", r.fileHandler.GetUploadSessionStatus)

		// 上传文件块，:sessionId 和 :index 是路径参数
		// POST /api/file/upload/some-uuid-string/0
		uploadGroup.POST("/:sessionId/:index", r.fileHandler.UploadChunk)

		// 删除上传会话
		// DELETE /api/file/upload
		uploadGroup.DELETE("", r.fileHandler.DeleteUploadSession)
	}

	// --- 文件夹专属路由组 ---
	folderGroup := api.Group("/folder")
	folderGroup.Use(r.mw.JWTAuth())
	{
		folderGroup.PUT("/view", r.fileHandler.UpdateFolderView)
		folderGroup.GET("/tree/:id", r.fileHandler.GetFolderTree)
		folderGroup.GET("/size/:id", r.fileHandler.GetFolderSize)
		folderGroup.POST("/move", r.fileHandler.MoveItems)
		folderGroup.POST("/copy", r.fileHandler.CopyItems)
	}
}

func (r *Router) registerDirectLinkRoutes(api *gin.RouterGroup) {
	// 这些操作需要用户登录，所以使用JWTAuth中间件
	directLinks := api.Group("/direct-links").Use(r.mw.JWTAuth())
	{
		// 注册创建直链的接口： POST /api/direct-links
		directLinks.POST("", r.directLinkHandler.GetOrCreateDirectLinks)

		// directLinks.GET("", r.directLinkHandler.ListMyDirectLinks)
		// directLinks.DELETE("/:id", r.directLinkHandler.DeleteDirectLink)
	}
}

func (r *Router) registerLinkRoutes(api *gin.RouterGroup) {
	// --- 前台公开接口 ---
	linksPublic := api.Group("/public/links")
	{
		// 申请友链: POST /api/public/links
		linksPublic.POST("", r.linkHandler.ApplyLink)

		// 获取公开友链列表: GET /api/public/links
		linksPublic.GET("", r.linkHandler.ListPublicLinks)

		// 获取随机友链: GET /api/public/links/random
		linksPublic.GET("/random", r.linkHandler.GetRandomLinks)
	}

	linkCategoriesPublic := api.Group("/public/link-categories")
	{
		// 获取友链分类列表: GET /api/public/link-categories
		linkCategoriesPublic.GET("", r.linkHandler.ListPublicCategories)
	}

	// --- 后台管理接口 ---
	linksAdmin := api.Group("/links").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		// 友链管理
		linksAdmin.POST("", r.linkHandler.AdminCreateLink)       // POST /api/links
		linksAdmin.GET("", r.linkHandler.ListLinks)              // GET /api/links
		linksAdmin.PUT("/:id", r.linkHandler.AdminUpdateLink)    // PUT /api/links/:id
		linksAdmin.DELETE("/:id", r.linkHandler.AdminDeleteLink) // DELETE /api/links/:id
		linksAdmin.PUT("/:id/review", r.linkHandler.ReviewLink)  // PUT /api/links/:id/review

		// 分类管理
		linksAdmin.GET("/categories", r.linkHandler.ListCategories)        // GET /api/links/categories
		linksAdmin.POST("/categories", r.linkHandler.CreateCategory)       // POST /api/links/categories
		linksAdmin.PUT("/categories/:id", r.linkHandler.UpdateCategory)    // PUT /api/links/categories/:id
		linksAdmin.DELETE("/categories/:id", r.linkHandler.DeleteCategory) // DELETE /api/links/categories/:id
		// 标签管理
		linksAdmin.GET("/tags", r.linkHandler.ListAllTags)      // GET /api/links/tags
		linksAdmin.POST("/tags", r.linkHandler.CreateTag)       // POST /api/links/tags
		linksAdmin.PUT("/tags/:id", r.linkHandler.UpdateTag)    // PUT /api/links/tags/:id
		linksAdmin.DELETE("/tags/:id", r.linkHandler.DeleteTag) // DELETE /api/links/tags/:id
	}
}

// registerStatisticsRoutes 注册统计相关的路由
func (r *Router) registerStatisticsRoutes(api *gin.RouterGroup) {
	// --- 前台公开接口 ---
	statisticsPublic := api.Group("/public/statistics")
	{
		// 获取基础统计数据: GET /api/public/statistics/basic
		statisticsPublic.GET("/basic", r.statisticsHandler.GetBasicStatistics)

		// 记录访问: POST /api/public/statistics/visit
		statisticsPublic.POST("/visit", r.statisticsHandler.RecordVisit)
	}

	// --- 后台管理接口 ---
	statisticsAdmin := api.Group("/statistics").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		// 获取访客分析数据: GET /api/statistics/analytics
		statisticsAdmin.GET("/analytics", r.statisticsHandler.GetVisitorAnalytics)

		// 获取热门页面: GET /api/statistics/top-pages
		statisticsAdmin.GET("/top-pages", r.statisticsHandler.GetTopPages)

		// 获取访客趋势数据: GET /api/statistics/trend
		statisticsAdmin.GET("/trend", r.statisticsHandler.GetVisitorTrend)

		// 获取统计概览: GET /api/statistics/summary
		statisticsAdmin.GET("/summary", r.statisticsHandler.GetStatisticsSummary)

		// 获取访客访问日志: GET /api/statistics/visitor-logs
		statisticsAdmin.GET("/visitor-logs", r.statisticsHandler.GetVisitorLogs)
	}
}

// registerSearchRoutes 注册搜索相关的路由
func (r *Router) registerSearchRoutes(api *gin.RouterGroup) {
	// 搜索接口是公开的，不需要认证
	searchGroup := api.Group("/search")
	{
		// 搜索文章: GET /api/search?q=关键词&page=1&size=10
		searchGroup.GET("", r.searchHandler.Search)
	}
}

// registerPageRoutes 注册页面相关的路由
func (r *Router) registerPageRoutes(api *gin.RouterGroup) {
	// --- 前台公开接口 ---
	pagesPublic := api.Group("/public/pages")
	{
		// 根据路径获取页面: GET /api/public/pages/:path
		pagesPublic.GET("/:path", r.pageHandler.GetByPath)
	}

	// --- 后台管理接口 ---
	pagesAdmin := api.Group("/pages").Use(r.mw.JWTAuth(), r.mw.AdminAuth())
	{
		// 页面管理
		pagesAdmin.POST("", r.pageHandler.Create)                            // POST /api/pages
		pagesAdmin.GET("", r.pageHandler.List)                               // GET /api/pages
		pagesAdmin.GET("/:id", r.pageHandler.GetByID)                        // GET /api/pages/:id
		pagesAdmin.PUT("/:id", r.pageHandler.Update)                         // PUT /api/pages/:id
		pagesAdmin.DELETE("/:id", r.pageHandler.Delete)                      // DELETE /api/pages/:id
		pagesAdmin.POST("/initialize", r.pageHandler.InitializeDefaultPages) // POST /api/pages/initialize
	}
}
