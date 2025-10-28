// internal/app/bootstrap/bootstrap.go
package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/link"
	"github.com/anzhiyu-c/anheyu-app/ent/linkcategory"
	"github.com/anzhiyu-c/anheyu-app/ent/setting"
	"github.com/anzhiyu-c/anheyu-app/ent/usergroup"
	"github.com/anzhiyu-c/anheyu-app/internal/configdef"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/utils"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

type Bootstrapper struct {
	entClient *ent.Client
}

func NewBootstrapper(entClient *ent.Client) *Bootstrapper {
	return &Bootstrapper{
		entClient: entClient,
	}
}

func (b *Bootstrapper) InitializeDatabase() error {
	log.Println("--- 开始执行数据库初始化引导程序 (配置注册表模式) ---")

	if err := b.entClient.Schema.Create(context.Background()); err != nil {
		return fmt.Errorf("数据库 schema 创建/更新失败: %w", err)
	}
	log.Println("--- 数据库 Schema 同步成功 ---")

	b.syncSettings()
	b.initUserGroups()
	b.initStoragePolicies()
	b.initLinks()
	b.initDefaultPages()
	b.checkUserTable()

	log.Println("--- 数据库初始化引导程序执行完成 ---")
	return nil
}

// syncSettings 检查并同步配置项，确保所有在代码中定义的配置项都存在于数据库中。
func (b *Bootstrapper) syncSettings() {
	log.Println("--- 开始同步站点配置 (Setting 表)... ---")
	ctx := context.Background()
	newlyAdded := 0

	// 从 configdef 循环所有定义
	for _, def := range configdef.AllSettings {
		exists, err := b.entClient.Setting.Query().Where(setting.ConfigKey(def.Key.String())).Exist(ctx)
		if err != nil {
			log.Printf("⚠️ 失败: 查询配置项 '%s' 失败: %v", def.Key, err)
			continue
		}

		// 如果配置项在数据库中不存在，则创建它
		if !exists {
			value := def.Value
			// 特殊处理需要动态生成的密钥
			if def.Key == constant.KeyJWTSecret {
				value, _ = utils.GenerateRandomString(32)
			}
			if def.Key == constant.KeyLocalFileSigningSecret {
				value, _ = utils.GenerateRandomString(32)
			}

			// 检查环境变量覆盖
			envKey := "AN_SETTING_DEFAULT_" + strings.ToUpper(string(def.Key))
			if envValue, ok := os.LookupEnv(envKey); ok {
				value = envValue
				log.Printf("    - 配置项 '%s' 由环境变量覆盖。", def.Key)
			}

			_, createErr := b.entClient.Setting.Create().
				SetConfigKey(def.Key.String()).
				SetValue(value).
				SetComment(def.Comment).
				Save(ctx)

			if createErr != nil {
				log.Printf("⚠️ 失败: 新增默认配置项 '%s' 失败: %v", def.Key, createErr)
			} else {
				log.Printf("    -新增配置项: '%s' 已写入数据库。", def.Key)
				newlyAdded++
			}
		}
	}

	if newlyAdded > 0 {
		log.Printf("--- 站点配置同步完成，共新增 %d 个配置项。---", newlyAdded)
	} else {
		log.Println("--- 站点配置同步完成，无需新增配置项。---")
	}
}

// initUserGroups 检查并初始化默认用户组。
func (b *Bootstrapper) initUserGroups() {
	log.Println("--- 开始初始化默认用户组 (UserGroup 表) ---")
	ctx := context.Background()
	for _, groupData := range configdef.AllUserGroups {
		exists, err := b.entClient.UserGroup.Query().Where(usergroup.ID(groupData.ID)).Exist(ctx)
		if err != nil {
			log.Printf("⚠️ 失败: 查询用户组 ID: %d 失败: %v", groupData.ID, err)
			continue
		}
		if !exists {
			_, createErr := b.entClient.UserGroup.Create().
				SetID(groupData.ID).
				SetName(groupData.Name).
				SetDescription(groupData.Description).
				SetPermissions(groupData.Permissions).
				SetMaxStorage(groupData.MaxStorage).
				SetSpeedLimit(groupData.SpeedLimit).
				SetSettings(&groupData.Settings).
				Save(ctx)
			if createErr != nil {
				log.Printf("⚠️ 失败: 创建默认用户组 '%s' (ID: %d) 失败: %v", groupData.Name, groupData.ID, createErr)
			}
		}
	}
	log.Println("--- 默认用户组 (UserGroup 表) 初始化完成。---")
}

func (b *Bootstrapper) initStoragePolicies() {
	log.Println("--- 开始初始化默认存储策略 (StoragePolicy 表) ---")
	ctx := context.Background()
	count, err := b.entClient.StoragePolicy.Query().Count(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 查询存储策略数量失败: %v", err)
		return
	}

	if count == 0 {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("❌ 致命错误: 无法获取当前工作目录: %v", err)
		}
		dirNameRule := filepath.Join(wd, "data/storage")

		settings := model.StoragePolicySettings{
			"chunk_size":    26214400,
			"pre_allocate":  true,
			"upload_method": constant.UploadMethodClient,
		}

		_, err = b.entClient.StoragePolicy.Create().
			SetName("本机存储").
			SetType(string(constant.PolicyTypeLocal)).
			SetBasePath(dirNameRule).
			SetVirtualPath("/").
			SetSettings(settings).
			Save(ctx)

		if err != nil {
			log.Printf("⚠️ 失败: 创建默认存储策略 '本机存储' 失败: %v", err)
		} else {
			log.Printf("✅ 成功: 默认存储策略 '本机存储' 已创建。路径规则: %s", dirNameRule)
		}
	}
	log.Println("--- 默认存储策略 (StoragePolicy 表) 初始化完成。---")
}

// initLinks 初始化友链、分类和标签表。
func (b *Bootstrapper) initLinks() {
	log.Println("--- 开始初始化友链模块 (Link, Category, Tag 表) ---")
	ctx := context.Background()

	count, err := b.entClient.Link.Query().Count(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 查询友链数量失败: %v", err)
		return
	}
	if count > 0 {
		log.Println("--- 友链模块已存在数据，跳过初始化。---")
		return
	}

	tx, err := b.entClient.Tx(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 启动友链初始化事务失败: %v", err)
		return
	}

	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	// --- 1. 创建默认分类 ---
	catTuijian, err := tx.LinkCategory.Create().
		SetName("推荐").
		SetStyle(linkcategory.StyleCard).
		SetDescription("优秀博主，综合排序。").
		Save(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 创建默认友链分类 '推荐' 失败: %v", tx.Rollback())
		return
	}
	if catTuijian.ID != 1 {
		log.Printf("🔥 严重警告: '推荐' 分类创建后的 ID 不是 1 (而是 %d)。", catTuijian.ID)
	}

	// 接着创建“小伙伴”，它会自动获得 ID=2
	catShuoban, err := tx.LinkCategory.Create().
		SetName("小伙伴").
		SetStyle(linkcategory.StyleList).
		SetDescription("那些人，那些事").
		Save(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 创建默认友链分类 '小伙伴' 失败: %v", tx.Rollback())
		return
	}
	// 健壮性检查：确认默认分类的 ID 确实是 2
	if catShuoban.ID != 2 {
		log.Printf("🔥 严重警告: 默认分类 '小伙伴' 创建后的 ID 不是 2 (而是 %d)。申请友链的默认分类功能可能不正常。", catShuoban.ID)
	}
	log.Println("    -默认分类 '推荐' 和 '小伙伴' 创建成功。")

	// --- 2. 创建默认标签 ---
	tagTech, err := tx.LinkTag.Create().
		SetName("技术").
		SetColor("linear-gradient(38deg,#e5b085 0,#d48f16 100%)").
		Save(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 创建默认友链标签 '技术' 失败: %v", tx.Rollback())
		return
	}
	_, err = tx.LinkTag.Create().
		SetName("生活").
		SetColor("var(--anzhiyu-green)").
		Save(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 创建默认友链标签 '生活' 失败: %v", tx.Rollback())
		return
	}
	log.Println("    -默认标签 '技术' 和 '生活' 创建成功。")

	// --- 3. 创建默认友链并关联 ---
	_, err = tx.Link.Create().
		SetName("安知鱼").
		SetURL("https://blog.anheyu.com/").
		SetLogo("https://npm.elemecdn.com/anzhiyu-blog-static@1.0.4/img/avatar.jpg").
		SetDescription("生活明朗，万物可爱").
		SetSiteshot("https://npm.elemecdn.com/anzhiyu-theme-static@1.1.6/img/blog.anheyu.com.jpg"). // 添加站点快照
		SetStatus(link.StatusAPPROVED).
		SetCategoryID(catTuijian.ID). // 关联到"推荐"分类 (ID=1)
		AddTagIDs(tagTech.ID).
		Save(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 创建默认友链 '安知鱼' 失败: %v", tx.Rollback())
		return
	}
	log.Println("    -默认友链 '安知鱼' (卡片样式) 创建成功。")

	// 创建第二个默认友链，使用list样式的分类
	_, err = tx.Link.Create().
		SetName("安知鱼").
		SetURL("https://blog.anheyu.com/").
		SetLogo("https://npm.elemecdn.com/anzhiyu-blog-static@1.0.4/img/avatar.jpg").
		SetDescription("生活明朗，万物可爱").
		SetStatus(link.StatusAPPROVED).
		SetCategoryID(catShuoban.ID).
		AddTagIDs(tagTech.ID).
		Save(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 创建默认友链 '安知鱼' (list样式) 失败: %v", tx.Rollback())
		return
	}
	log.Println("    -默认友链 '安知鱼' (列表样式) 创建成功。")

	if err := tx.Commit(); err != nil {
		log.Printf("⚠️ 失败: 提交友链初始化事务失败: %v", err)
		return
	}

	log.Println("--- 友链模块初始化完成。---")
}

func (b *Bootstrapper) checkUserTable() {
	ctx := context.Background()
	userCount, err := b.entClient.User.Query().Count(ctx)
	if err != nil {
		log.Printf("❌ 错误: 查询 User 表记录数量失败: %v", err)
	} else if userCount == 0 {
		log.Println("User 表为空，第一个注册的用户将成为管理员。")
	}
}

// initDefaultPages 检查并初始化默认页面
func (b *Bootstrapper) initDefaultPages() {
	log.Println("--- 开始初始化默认页面 (Page 表) ---")
	ctx := context.Background()

	// 检查是否已有页面数据
	pageCount, err := b.entClient.Page.Query().Count(ctx)
	if err != nil {
		log.Printf("⚠️ 失败: 查询页面数量失败: %v", err)
		return
	}

	if pageCount > 0 {
		log.Printf("--- 页面表已有 %d 条数据，跳过默认页面初始化。---", pageCount)
		return
	}

	// 定义默认页面
	defaultPages := []struct {
		title       string
		path        string
		content     string
		description string
		isPublished bool
		sort        int
	}{
		{
			title: "隐私政策",
			path:  "/privacy",
			content: `<div class="container mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold mb-6">隐私政策</h1>
    <div class="prose max-w-none">
        <p>本隐私政策描述了本站如何收集、使用和保护您的个人信息。</p>
        <h2>信息收集</h2>
        <p>我们可能收集以下类型的信息：</p>
        <ul>
            <li>您主动提供的信息</li>
            <li>自动收集的技术信息</li>
            <li>第三方来源的信息</li>
        </ul>
        <h2>信息使用</h2>
        <p>我们使用收集的信息来：</p>
        <ul>
            <li>提供和改进服务</li>
            <li>个性化用户体验</li>
            <li>发送通知和更新</li>
        </ul>
        <h2>信息保护</h2>
        <p>我们采取适当的安全措施来保护您的个人信息。</p>
    </div>
</div>`,
			description: "本站的隐私政策说明",
			isPublished: true,
			sort:        1,
		},
		{
			title: "Cookie 政策",
			path:  "/cookies",
			content: `<div class="container mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold mb-6">Cookie 政策</h1>
    <div class="prose max-w-none">
        <p>本Cookie政策说明了本站如何使用Cookie和类似技术。</p>
        <h2>什么是Cookie</h2>
        <p>Cookie是存储在您设备上的小型文本文件，用于记住您的偏好设置和登录状态。</p>
        <h2>我们使用的Cookie类型</h2>
        <ul>
            <li><strong>必要Cookie</strong>：网站正常运行所必需</li>
            <li><strong>功能Cookie</strong>：记住您的偏好设置</li>
            <li><strong>分析Cookie</strong>：帮助我们了解网站使用情况</li>
        </ul>
        <h2>管理Cookie</h2>
        <p>您可以通过浏览器设置来管理Cookie，但禁用某些Cookie可能会影响网站功能。</p>
    </div>
</div>`,
			description: "本站的Cookie使用政策",
			isPublished: true,
			sort:        2,
		},
		{
			title: "版权声明",
			path:  "/copyright",
			content: `<div class="container mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold mb-6">版权声明</h1>
    <div class="prose max-w-none">
        <p>本版权声明适用于本站的所有内容。</p>
        <h2>版权保护</h2>
        <p>本站的所有内容，包括但不限于文字、图片、音频、视频等，均受版权法保护。</p>
        <h2>使用许可</h2>
        <p>未经许可，禁止复制、分发、展示或创建衍生作品。</p>
        <h2>例外情况</h2>
        <ul>
            <li>合理使用（如评论、教育、研究等）</li>
            <li>获得明确书面许可</li>
            <li>内容已进入公共领域</li>
        </ul>
        <h2>联系我们</h2>
        <p>如果您需要获得使用许可或有其他版权相关问题，请联系我们。</p>
    </div>
</div>`,
			description: "本站的版权保护声明",
			isPublished: true,
			sort:        3,
		},
	}

	// 创建默认页面
	createdCount := 0
	for _, pageData := range defaultPages {
		_, err := b.entClient.Page.Create().
			SetTitle(pageData.title).
			SetPath(pageData.path).
			SetContent(pageData.content).
			SetDescription(pageData.description).
			SetIsPublished(pageData.isPublished).
			SetSort(pageData.sort).
			Save(ctx)

		if err != nil {
			log.Printf("⚠️ 失败: 创建默认页面 '%s' 失败: %v", pageData.title, err)
		} else {
			log.Printf("    -默认页面 '%s' (%s) 创建成功。", pageData.title, pageData.path)
			createdCount++
		}
	}

	if createdCount > 0 {
		log.Printf("--- 默认页面初始化完成，共创建 %d 个页面。---", createdCount)
	} else {
		log.Println("--- 默认页面初始化失败，未创建任何页面。---")
	}
}
