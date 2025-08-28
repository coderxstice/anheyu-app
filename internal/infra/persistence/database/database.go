/*
 * @Description: 数据库连接管理 (支持多种数据库)
 * @Author: 安知鱼
 * @Date: 2025-07-12 16:09:46
 * @LastEditTime: 2025-07-29 16:00:31
 * @LastEditors: 安知鱼
 */
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/pkg/config"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// NewSQLDB 创建并返回一个标准的 *sql.DB 连接池，现在支持多种数据库。
func NewSQLDB(cfg *config.Config) (*sql.DB, error) {
	driver := cfg.GetString(config.KeyDBType)
	if driver == "" {
		log.Println("警告: 配置文件中未指定 'Database.Type'，将默认使用 'mysql'")
		driver = "mysql"
	}

	var dsn string
	var driverName string

	dbUser := cfg.GetString(config.KeyDBUser)
	dbPass := cfg.GetString(config.KeyDBPassword)
	dbHost := cfg.GetString(config.KeyDBHost)
	dbPort := cfg.GetString(config.KeyDBPort)
	dbName := cfg.GetString(config.KeyDBName)

	switch driver {
	case "mysql":
		driverName = "mysql"
		if dbUser == "" || dbHost == "" || dbPort == "" || dbName == "" {
			return nil, fmt.Errorf("MySQL 连接参数不完整 (需要 User, Host, Port, Name)")
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dbUser, dbPass, dbHost, dbPort, dbName)
	case "postgres":
		driverName = "postgres"
		if dbUser == "" || dbHost == "" || dbPort == "" || dbName == "" {
			return nil, fmt.Errorf("PostgreSQL 连接参数不完整 (需要 User, Host, Port, Name)")
		}
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPass, dbName)
	case "sqlite", "sqlite3":
		driverName = "sqlite3"

		dataDir := "./data"
		if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("无法创建 data 目录: %w", err)
		}

		finalDbName := dbName
		if finalDbName == "" {
			finalDbName = "anheyu_app.db" // 如果未指定数据库名，则使用默认值
		}

		finalPath := filepath.Join(dataDir, finalDbName)
		log.Printf("【提示】SQLite 数据库路径: %s\n", finalPath)

		// 使用 file: DSN 格式并启用外键约束
		dsn = fmt.Sprintf("file:%s?_fk=1&cache=shared", finalPath)
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s (支持: mysql, postgres, sqlite)", driver)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 sql.DB 连接失败 (驱动: %s): %w", driverName, err)
	}

	// 设置连接池参数
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)

	// 验证数据库连接
	if err := db.Ping(); err != nil {
		db.Close() // 如果 ping 失败，关闭连接以释放资源
		return nil, fmt.Errorf("无法 Ping 通数据库 (DSN: %s): %w", dsn, err)
	}

	log.Printf("✅ %s 数据库连接池创建成功！\n", strings.Title(driver))
	return db, nil
}

// NewEntClient 根据配置创建并返回一个 Ent ORM 客户端。
func NewEntClient(db *sql.DB, cfg *config.Config) (*ent.Client, error) {
	// *FIXED*: 使用 KeyDBType 来获取数据库类型，以匹配 conf.ini 的配置
	driverName := cfg.GetString(config.KeyDBType)
	if driverName == "" {
		driverName = "mysql" // 保持与 NewSQLDB 的默认值一致
	}

	var drv dialect.Driver
	switch driverName {
	case "mysql":
		drv = entsql.OpenDB(dialect.MySQL, db)
	case "postgres":
		drv = entsql.OpenDB(dialect.Postgres, db)
	case "sqlite", "sqlite3":
		drv = entsql.OpenDB(dialect.SQLite, db)
	default:
		return nil, fmt.Errorf("不支持的 Ent 方言: %s", driverName)
	}

	var entOptions []ent.Option

	// 1. 始终添加 Driver 选项
	entOptions = append(entOptions, ent.Driver(drv))

	// 2. 根据配置决定是否添加 Debug 选项
	if cfg.GetBool(config.KeyDBDebug) {
		entOptions = append(entOptions, ent.Debug())
		log.Println("【数据库】Ent Debug模式已开启，将打印所有执行的SQL语句。")
	}

	// 使用所有收集到的选项创建客户端
	client := ent.NewClient(entOptions...)

	// 在开发/启动阶段自动同步数据库结构
	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, fmt.Errorf("创建/更新数据库 schema 失败 (Ent): %w", err)
	}

	log.Println("✅ Ent 客户端初始化成功，并已同步数据库 schema！")

	// 额外校验：检查 post_categories.is_series 列是否存在，便于定位迁移问题
	if err := verifyPostCategoriesIsSeriesColumn(db, driverName); err != nil {
		log.Printf("⚠️ 列存在性校验时发生错误: %v", err)
	}
	return client, nil
}

// verifyPostCategoriesIsSeriesColumn 检查表 post_categories 上是否存在 is_series 列，并打印详细结果。
func verifyPostCategoriesIsSeriesColumn(db *sql.DB, driverName string) error {
	switch driverName {
	case "postgres":
		// 默认 schema=public
		const q = `SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'post_categories' AND column_name = 'is_series'`
		row := db.QueryRow(q)
		var name, dataType, isNullable, colDefault string
		if err := row.Scan(&name, &dataType, &isNullable, &colDefault); err != nil {
			if err == sql.ErrNoRows {
				log.Println("❌ 校验结果: 表 post_categories 缺少列 is_series (postgres)")
				return nil
			}
			return fmt.Errorf("postgres 校验查询失败: %w", err)
		}
		log.Printf("✅ 校验结果: 列存在 (postgres) -> name=%s, type=%s, nullable=%s, default=%s", name, dataType, isNullable, colDefault)
		return nil
	case "mysql":
		const q = `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'post_categories' AND COLUMN_NAME = 'is_series'`
		row := db.QueryRow(q)
		var name, dataType, isNullable, colDefault sql.NullString
		if err := row.Scan(&name, &dataType, &isNullable, &colDefault); err != nil {
			if err == sql.ErrNoRows {
				log.Println("❌ 校验结果: 表 post_categories 缺少列 is_series (mysql)")
				return nil
			}
			return fmt.Errorf("mysql 校验查询失败: %w", err)
		}
		log.Printf("✅ 校验结果: 列存在 (mysql) -> name=%s, type=%s, nullable=%s, default=%s", name.String, dataType.String, isNullable.String, colDefault.String)
		return nil
	case "sqlite", "sqlite3":
		const q = "PRAGMA table_info('post_categories')"
		rows, err := db.Query(q)
		if err != nil {
			return fmt.Errorf("sqlite 校验查询失败: %w", err)
		}
		defer rows.Close()
		exists := false
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				return fmt.Errorf("sqlite 扫描失败: %w", err)
			}
			if name == "is_series" {
				log.Printf("✅ 校验结果: 列存在 (sqlite) -> name=%s, type=%s, notnull=%d, default=%s, pk=%d", name, typ, notnull, dflt.String, pk)
				exists = true
				break
			}
		}
		if !exists {
			log.Println("❌ 校验结果: 表 post_categories 缺少列 is_series (sqlite)")
		}
		return nil
	default:
		log.Printf("(跳过校验) 未知/不支持的驱动用于列校验: %s", driverName)
		return nil
	}
}
