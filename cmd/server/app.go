// anheyu-app/cmd/server/app.go
package server

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/anzhiyu-c/anheyu-app/internal/app/bootstrap"
	"github.com/anzhiyu-c/anheyu-app/internal/app/listener"
	"github.com/anzhiyu-c/anheyu-app/internal/app/middleware"
	"github.com/anzhiyu-c/anheyu-app/internal/app/task"
	"github.com/anzhiyu-c/anheyu-app/internal/infra/persistence/database"
	ent_impl "github.com/anzhiyu-c/anheyu-app/internal/infra/persistence/ent"
	"github.com/anzhiyu-c/anheyu-app/internal/infra/router"
	"github.com/anzhiyu-c/anheyu-app/internal/infra/storage"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/event"
	"github.com/anzhiyu-c/anheyu-app/pkg/config"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	album_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/album"
	article_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/article"
	auth_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/auth"
	comment_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/comment"
	direct_link_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/direct_link"
	file_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/file"
	link_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/link"
	post_category_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/post_category"
	post_tag_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/post_tag"
	public_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/public"
	setting_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/setting"
	statistics_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/statistics"
	storage_policy_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/storage_policy"
	thumbnail_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/thumbnail"
	user_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/user"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/album"
	article_service "github.com/anzhiyu-c/anheyu-app/pkg/service/article"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/auth"
	cleanup_service "github.com/anzhiyu-c/anheyu-app/pkg/service/cleanup"
	comment_service "github.com/anzhiyu-c/anheyu-app/pkg/service/comment"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/direct_link"
	file_service "github.com/anzhiyu-c/anheyu-app/pkg/service/file"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/file_info"
	link_service "github.com/anzhiyu-c/anheyu-app/pkg/service/link"
	parser_service "github.com/anzhiyu-c/anheyu-app/pkg/service/parser"
	post_category_service "github.com/anzhiyu-c/anheyu-app/pkg/service/post_category"
	post_tag_service "github.com/anzhiyu-c/anheyu-app/pkg/service/post_tag"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/process"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/statistics"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/thumbnail"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/user"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/volume"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/volume/strategy"

	_ "github.com/anzhiyu-c/anheyu-app/ent/runtime"
)

// App 结构体，用于封装应用的所有核心组件
type App struct {
	cfg                  *config.Config
	engine               *gin.Engine
	taskBroker           *task.Broker
	sqlDB                *sql.DB
	articleService       article_service.Service
	directLinkService    direct_link.Service
	storagePolicyRepo    repository.StoragePolicyRepository
	storagePolicyService volume.IStoragePolicyService
	mw                   *middleware.Middleware
	settingSvc           setting.SettingService
	fileRepo             repository.FileRepository
}

// printBanner 打印应用启动 banner
func (a *App) printBanner() {
	banner := `

       █████╗ ███╗   ██╗███████╗██╗  ██╗██╗██╗   ██╗██╗   ██╗
      ██╔══██╗████╗  ██║╚══███╔╝██║  ██║██║╚██╗ ██╔╝██║   ██║
      ███████║██╔██╗ ██║  ███╔╝ ███████║██║ ╚████╔╝ ██║   ██║
      ██╔══██║██║╚██╗██║ ███╔╝  ██╔══██║██║  ╚██╔╝  ██║   ██║
      ██║  ██║██║ ╚████║███████╗██║  ██║██║   ██║   ╚██████╔╝
      ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝╚═╝   ╚═╝    ╚═════╝

`
	log.Println(banner)
	log.Println("--------------------------------------------------------")

	// 检查是否为 PRO 版本
	if os.Getenv("ANHEYU_LICENSE_KEY") != "" {
		log.Println(" Anheyu App - PRO 专业版")
	} else {
		log.Println(" Anheyu App - Community Version")
	}
	log.Println("--------------------------------------------------------")
}

// NewApp 是应用的构造函数，它执行所有的初始化和依赖注入工作
func NewApp(content embed.FS) (*App, func(), error) {
	// --- Phase 1: 加载外部配置 ---
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// --- Phase 2: 初始化基础设施 ---
	sqlDB, err := database.NewSQLDB(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("创建数据库连接池失败: %w", err)
	}
	entClient, err := database.NewEntClient(sqlDB, cfg)
	if err != nil {
		sqlDB.Close()
		return nil, nil, err
	}
	redisClient, err := database.NewRedisClient(context.Background(), cfg)
	if err != nil {
		sqlDB.Close()
		return nil, nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}
	cleanup := func() {
		log.Println("执行清理操作：关闭数据库和Redis连接...")
		sqlDB.Close()
		redisClient.Close()
	}
	if err := idgen.InitSqidsEncoder(); err != nil {
		return nil, cleanup, fmt.Errorf("初始化 ID 编码器失败: %w", err)
	}
	eventBus := event.NewEventBus()
	dbType := cfg.GetString(config.KeyDBType)
	if dbType == "" {
		dbType = "mysql"
	}

	// --- Phase 3: 初始化数据仓库层 ---
	settingRepo := ent_impl.NewEntSettingRepository(entClient)
	userRepo := ent_impl.NewEntUserRepository(entClient)
	userGroupRepo := ent_impl.NewEntUserGroupRepository(entClient)
	fileRepo := ent_impl.NewEntFileRepository(entClient, sqlDB, dbType)
	entityRepo := ent_impl.NewEntEntityRepository(entClient)
	fileEntityRepo := ent_impl.NewEntFileEntityRepository(entClient)
	tagRepo := ent_impl.NewEntTagRepository(entClient)
	directLinkRepo := ent_impl.NewEntDirectLinkRepository(entClient)
	albumRepo := ent_impl.NewEntAlbumRepository(entClient)
	storagePolicyRepo := ent_impl.NewEntStoragePolicyRepository(entClient)
	metadataRepo := ent_impl.NewEntMetadataRepository(entClient)
	articleRepo := ent_impl.NewArticleRepo(entClient)
	postTagRepo := ent_impl.NewPostTagRepo(entClient, dbType)
	postCategoryRepo := ent_impl.NewPostCategoryRepo(entClient)
	cleanupRepo := ent_impl.NewCleanupRepo(entClient)
	commentRepo := ent_impl.NewCommentRepo(entClient, dbType)
	linkRepo := ent_impl.NewLinkRepo(entClient, dbType)
	linkCategoryRepo := ent_impl.NewLinkCategoryRepo(entClient)
	linkTagRepo := ent_impl.NewLinkTagRepo(entClient)

	// --- Phase 4: 初始化应用引导程序 ---
	bootstrapper := bootstrap.NewBootstrapper(entClient)
	if err := bootstrapper.InitializeDatabase(); err != nil {
		return nil, cleanup, fmt.Errorf("数据库初始化失败: %w", err)
	}

	// --- Phase 5: 初始化业务逻辑层 ---
	// 注意：这里的 Svc 变量现在都是接口类型，因为它们的构造函数返回接口
	txManager := ent_impl.NewEntTransactionManager(entClient, sqlDB, dbType)
	settingSvc := setting.NewSettingService(settingRepo, eventBus)
	if err := settingSvc.LoadAllSettings(context.Background()); err != nil {
		return nil, cleanup, fmt.Errorf("从数据库加载站点配置失败: %w", err)
	}
	strategyManager := strategy.NewManager()
	strategyManager.Register(constant.PolicyTypeLocal, strategy.NewLocalStrategy())
	strategyManager.Register(constant.PolicyTypeOneDrive, strategy.NewOneDriveStrategy())
	emailSvc := utility.NewEmailService(settingSvc)
	cacheSvc := utility.NewCacheService(redisClient)
	tokenSvc := auth.NewTokenService(userRepo, settingSvc, cacheSvc)
	geoSvc, err := utility.NewGeoIPService(settingSvc)
	if err != nil {
		log.Printf("警告: GeoIP 服务初始化失败: %v。IP属地将显示为'未知'", err)
	}
	albumSvc := album.NewAlbumService(albumRepo, tagRepo, settingSvc)
	storageProviders := make(map[constant.StoragePolicyType]storage.IStorageProvider)
	localSigningSecret := settingSvc.Get(constant.KeyLocalFileSigningSecret.String())
	parserSvc := parser_service.NewService(settingSvc, eventBus)
	storageProviders[constant.PolicyTypeLocal] = storage.NewLocalProvider(localSigningSecret)
	storageProviders[constant.PolicyTypeOneDrive] = storage.NewOneDriveProvider(storagePolicyRepo)
	metadataSvc := file_info.NewMetadataService(metadataRepo)
	postTagSvc := post_tag_service.NewService(postTagRepo)
	postCategorySvc := post_category_service.NewService(postCategoryRepo)
	cleanupSvc := cleanup_service.NewCleanupService(cleanupRepo)
	userSvc := user.NewUserService(userRepo)
	storagePolicySvc := volume.NewStoragePolicyService(storagePolicyRepo, fileRepo, txManager, strategyManager, settingSvc, cacheSvc)
	thumbnailSvc := thumbnail.NewThumbnailService(metadataSvc, fileRepo, entityRepo, storagePolicySvc, settingSvc, storageProviders)
	if err != nil {
		return nil, cleanup, fmt.Errorf("初始化缩略图服务失败: %w", err)
	}
	pathLocker := utility.NewPathLocker()
	syncSvc := process.NewSyncService(txManager, fileRepo, entityRepo, fileEntityRepo, storagePolicySvc, eventBus, storageProviders)
	vfsSvc := volume.NewVFSService(storagePolicySvc, cacheSvc, fileRepo, entityRepo, settingSvc, storageProviders)
	extractionSvc := file_info.NewExtractionService(fileRepo, settingSvc, metadataSvc, vfsSvc)
	fileSvc := file_service.NewService(fileRepo, storagePolicyRepo, txManager, entityRepo, fileEntityRepo, metadataSvc, extractionSvc, cacheSvc, storagePolicySvc, settingSvc, syncSvc, vfsSvc, storageProviders, eventBus, pathLocker)
	uploadSvc := file_service.NewUploadService(txManager, eventBus, entityRepo, metadataSvc, cacheSvc, storagePolicySvc, settingSvc, storageProviders)
	directLinkSvc := direct_link.NewDirectLinkService(directLinkRepo, fileRepo, userGroupRepo, settingSvc, storagePolicyRepo)
	statService, err := statistics.NewVisitorStatService(
		ent_impl.NewVisitorStatRepository(entClient),
		ent_impl.NewVisitorLogRepository(entClient),
		ent_impl.NewURLStatRepository(entClient),
		cacheSvc,
		geoSvc,
	)
	if err != nil {
		return nil, cleanup, fmt.Errorf("初始化统计服务失败: %w", err)
	}
	taskBroker := task.NewBroker(uploadSvc, thumbnailSvc, cleanupSvc, articleRepo, commentRepo, emailSvc, cacheSvc, linkCategoryRepo, linkTagRepo, settingSvc, statService)
	linkSvc := link_service.NewService(linkRepo, linkCategoryRepo, linkTagRepo, txManager, taskBroker)
	articleSvc := article_service.NewService(articleRepo, postTagRepo, postCategoryRepo, txManager, cacheSvc, geoSvc, taskBroker, settingSvc, parserSvc, fileSvc, directLinkSvc)
	authSvc := auth.NewAuthService(userRepo, settingSvc, tokenSvc, emailSvc, txManager, articleSvc)
	commentSvc := comment_service.NewService(commentRepo, userRepo, txManager, geoSvc, settingSvc, cacheSvc, taskBroker, fileSvc, parserSvc)
	_ = listener.NewFilePostProcessingListener(eventBus, taskBroker, extractionSvc)

	// --- Phase 6: 初始化表现层 (Handlers) ---
	mw := middleware.NewMiddleware(tokenSvc)
	authHandler := auth_handler.NewAuthHandler(authSvc, tokenSvc, settingSvc)
	albumHandler := album_handler.NewAlbumHandler(albumSvc)
	userHandler := user_handler.NewUserHandler(userSvc, settingSvc)
	publicHandler := public_handler.NewPublicHandler(albumSvc)
	settingHandler := setting_handler.NewSettingHandler(settingSvc, emailSvc)
	storagePolicyHandler := storage_policy_handler.NewStoragePolicyHandler(storagePolicySvc)
	fileHandler := file_handler.NewHandler(fileSvc, uploadSvc, settingSvc)
	directLinkHandler := direct_link_handler.NewDirectLinkHandler(directLinkSvc, storageProviders)
	linkHandler := link_handler.NewHandler(linkSvc)
	thumbnailHandler := thumbnail_handler.NewThumbnailHandler(taskBroker, metadataSvc, fileSvc, thumbnailSvc, settingSvc)
	articleHandler := article_handler.NewHandler(articleSvc)
	postTagHandler := post_tag_handler.NewHandler(postTagSvc)
	postCategoryHandler := post_category_handler.NewHandler(postCategorySvc)
	commentHandler := comment_handler.NewHandler(commentSvc)
	statisticsHandler := statistics_handler.NewStatisticsHandler(statService)

	// --- Phase 7: 初始化路由 ---
	appRouter := router.NewRouter(
		authHandler,
		albumHandler,
		userHandler,
		publicHandler,
		settingHandler,
		storagePolicyHandler,
		fileHandler,
		directLinkHandler,
		thumbnailHandler,
		articleHandler,
		postTagHandler,
		postCategoryHandler,
		commentHandler,
		linkHandler,
		statisticsHandler,
		mw,
	)

	// --- Phase 8: 配置 Gin 引擎 ---
	engine := gin.Default()
	err = engine.SetTrustedProxies(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("设置信任代理失败: %w", err)
	}
	engine.ForwardedByClientIP = true
	engine.Use(middleware.Cors())
	router.SetupFrontend(engine, settingSvc, articleSvc, content)
	appRouter.Setup(engine)

	// 将所有初始化好的组件装配到 App 实例中
	app := &App{
		cfg:                  cfg,
		engine:               engine,
		taskBroker:           taskBroker,
		sqlDB:                sqlDB,
		articleService:       articleSvc,
		directLinkService:    directLinkSvc,
		storagePolicyRepo:    storagePolicyRepo,
		storagePolicyService: storagePolicySvc,
		mw:                   mw,
		settingSvc:           settingSvc,
		fileRepo:             fileRepo,
	}

	return app, cleanup, nil
}

func (a *App) Config() *config.Config {
	return a.cfg
}

func (a *App) Engine() *gin.Engine {
	return a.engine
}

func (a *App) FileRepository() repository.FileRepository {
	return a.fileRepo
}

func (a *App) SettingService() setting.SettingService {
	return a.settingSvc
}

func (a *App) Middleware() *middleware.Middleware {
	return a.mw
}

func (a *App) ArticleService() article_service.Service {
	return a.articleService
}

func (a *App) DirectLinkService() direct_link.Service {
	return a.directLinkService
}

func (a *App) StoragePolicyRepository() repository.StoragePolicyRepository {
	return a.storagePolicyRepo
}

func (a *App) DB() *sql.DB {
	return a.sqlDB
}

func (a *App) StoragePolicyService() volume.IStoragePolicyService {
	return a.storagePolicyService
}

func (a *App) Run() error {
	a.taskBroker.RegisterCronJobs()
	a.taskBroker.CheckAndRunMissedAggregation()
	a.taskBroker.Start()
	port := a.cfg.GetString(config.KeyServerPort)
	if port == "" {
		port = "8091"
	}
	fmt.Printf("应用程序启动成功，正在监听端口: %s\n", port)

	// 在启动服务器前打印 banner
	a.printBanner()

	return a.engine.Run(":" + port)
}

func (a *App) Stop() {
	if a.taskBroker != nil {
		a.taskBroker.Stop()
		log.Println("任务调度器已停止。")
	}
}
