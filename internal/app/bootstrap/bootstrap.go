// internal/app/bootstrap/bootstrap.go
package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"anheyu-app/ent"
	"anheyu-app/ent/link"
	"anheyu-app/ent/linkcategory" // 导入 linkcategory 包
	"anheyu-app/ent/setting"
	"anheyu-app/ent/usergroup"
	"anheyu-app/internal/configdef"
	"anheyu-app/internal/constant"
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/pkg/utils"
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
				log.Printf("    - ✅ 新增配置项: '%s' 已写入数据库。", def.Key)
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
	log.Println("    - ✅ 默认分类 '推荐' 和 '小伙伴' 创建成功。")

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
	log.Println("    - ✅ 默认标签 '技术' 和 '生活' 创建成功。")

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
	log.Println("    - ✅ 默认友链 '安知鱼' (卡片样式) 创建成功。")

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
	log.Println("    - ✅ 默认友链 '安知鱼' (列表样式) 创建成功。")

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
