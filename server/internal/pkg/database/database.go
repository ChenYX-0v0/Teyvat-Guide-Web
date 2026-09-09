package database

import (
	"fmt"
	"os"
	"path/filepath"

	"teyvat-guide/server/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var db *gorm.DB

// Init 初始化数据库连接
// 支持驱动：sqlite（pure-go 默认）、mysql（DSN 为空时用 host/port/user/password/name 拼装）
func Init(cfg config.DatabaseConfig) error {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "sqlite":
		if err := ensureDir(cfg.DSN); err != nil {
			return err
		}
		dialector = sqlite.Open(cfg.DSN)
	case "mysql":
		dsn := cfg.DSN
		if dsn == "" {
			if err := ensureMySQLDatabase(cfg); err != nil {
				return fmt.Errorf("prepare database %s: %w", cfg.Name, err)
			}
			dsn = buildMySQLDSN(cfg, cfg.Name)
		}
		dialector = mysql.Open(dsn)
	default:
		return nil
	}

	conn, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return err
	}
	db = conn
	return nil
}

// DB 获取数据库实例
func DB() *gorm.DB {
	return db
}

// buildMySQLDSN 拼装 go-sql-driver 的 DSN；name 为空时连接到服务器（不选库）
func buildMySQLDSN(cfg config.DatabaseConfig, name string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, name)
}

// ensureMySQLDatabase 自动创建目标库（utf8mb4）
func ensureMySQLDatabase(cfg config.DatabaseConfig) error {
	if cfg.Name == "" {
		return nil
	}

	server, err := gorm.Open(mysql.Open(buildMySQLDSN(cfg, "")), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return err
	}
	if sqlDB, dbErr := server.DB(); dbErr == nil {
		defer sqlDB.Close() //nolint:errcheck
	}

	return server.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		cfg.Name,
	)).Error
}

func ensureDir(dsn string) error {
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
