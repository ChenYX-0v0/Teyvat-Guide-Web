package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用全局配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	AI       AIConfig       `mapstructure:"ai"`
	Auth     AuthConfig     `mapstructure:"auth"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug | release
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	Mode  string `mapstructure:"mode"` // console | json
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"` // sqlite | mysql | ""
	DSN      string `mapstructure:"dsn"`    // 完整 DSN；为空时用下方字段拼装（mysql）
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
}

type AIConfig struct {
	Provider string `mapstructure:"provider"` // openai | mock
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
	Model    string `mapstructure:"model"`
	System   string `mapstructure:"system"`
}

type AuthConfig struct {
	JWTSecret   string `mapstructure:"jwt_secret"`
	JWTTTLHours int    `mapstructure:"jwt_ttl_hours"` // token 有效期（小时）
	SeedUser    string `mapstructure:"seed_user"`     // 首次启动种子账号（用户表为空时自动创建）
	SeedPass    string `mapstructure:"seed_pass"`
	AdminKey    string `mapstructure:"admin_key"` // 管理接口密钥（X-Admin-Key）；为空时回落 jwt_secret
}

// AdminAPIKey 返回管理接口实际使用的密钥：优先 admin_key，未配置则复用 jwt_secret（不推荐）
func (c AuthConfig) AdminAPIKey() string {
	if c.AdminKey != "" {
		return c.AdminKey
	}
	return c.JWTSecret
}

// Load 读取配置文件 + 环境变量（环境变量优先级更高）
func Load() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath("../config")
	v.AddConfigPath(".")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetEnvPrefix("TEYVAT")

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			fmt.Fprintln(os.Stderr, "[config] 未找到 config.yaml，使用内置默认值")
		} else {
			// 解析失败同样退化为默认值，但必须显式暴露，否则配置项会莫名失效
			fmt.Fprintf(os.Stderr, "[config] config.yaml 加载失败，使用内置默认值: %v\n", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		panic(err)
	}
	return cfg
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.mode", "console")
	v.SetDefault("database.driver", "mysql")
	v.SetDefault("database.host", "127.0.0.1")
	v.SetDefault("database.port", "3306")
	v.SetDefault("database.user", "root")
	v.SetDefault("database.password", "root")
	v.SetDefault("database.name", "teyvat_guide")
	v.SetDefault("database.dsn", "")
	v.SetDefault("ai.provider", "mock")
	v.SetDefault("ai.base_url", "https://api.openai.com/v1")
	v.SetDefault("ai.model", "gpt-4o-mini")
	v.SetDefault("ai.system", "你是「AI 提瓦特向导」，一位熟悉《原神》的资深玩家助手，回答简洁、准确、友好。")
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.jwt_ttl_hours", 72)
	v.SetDefault("auth.seed_user", "admin")
	v.SetDefault("auth.seed_pass", "admin123")
}
