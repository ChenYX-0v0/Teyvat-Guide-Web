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
	Quota    QuotaConfig    `mapstructure:"quota"`
}

// QuotaConfig 用量配额与并发闸门（口径真源：shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §2）
// 只在主服务侧强制：AI 服务只认一把 X-AI-Key，无法区分用户。
type QuotaConfig struct {
	// DailyTokensPerUser 每用户每自然日（本地时区）token 上限；0 = 不限制
	DailyTokensPerUser int `mapstructure:"daily_tokens_per_user"`
	// DailyTokensGlobal 全站当日 token 上限（熔断，防批量注册绕过单用户配额）；0 = 关闭
	DailyTokensGlobal int `mapstructure:"daily_tokens_global"`
	// MaxConcurrentPerUser 同一用户同时进行中的 AI 请求上限；0 = 不限制
	// （AI 服务侧是全局信号量 LLM_MAX_CONCURRENCY，一人打满会让所有人排队）
	MaxConcurrentPerUser int `mapstructure:"max_concurrent_per_user"`
	// WhitelistUserIDs 豁免用户（默认空）
	WhitelistUserIDs []uint `mapstructure:"whitelist_user_ids"`
	// Metric 计量口径：total（默认，SUM(tokens)）| cached_weighted（预留：cached 按 1/50 加权）
	Metric string `mapstructure:"metric"`
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
	Provider       string `mapstructure:"provider"` // mock | openai | remote
	APIKey         string `mapstructure:"api_key"`  // remote 模式下为 AI 服务密钥（X-AI-Key）
	BaseURL        string `mapstructure:"base_url"` // remote 模式下指向 AI 服务，如 http://127.0.0.1:8090
	Model          string `mapstructure:"model"`    // 留空则由 AI 服务按其默认路由选择
	System         string `mapstructure:"system"`   // 留空则使用 AI 服务的人格提示词
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type AuthConfig struct {
	JWTSecret   string `mapstructure:"jwt_secret"`
	JWTTTLHours int    `mapstructure:"jwt_ttl_hours"` // token 有效期（小时）
	SeedUser    string `mapstructure:"seed_user"`     // 首次启动种子账号（用户表为空时自动创建）
	SeedPass    string `mapstructure:"seed_pass"`
	AdminKey    string `mapstructure:"admin_key"`    // 管理接口密钥（X-Admin-Key）；为空时回落 jwt_secret
	InternalKey string `mapstructure:"internal_key"` // 内网管理接口密钥（X-Internal-Key）；为空则内网接口不注册（安全默认）
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
	v.SetDefault("ai.timeout_seconds", 65) // 必须大于 AI 服务自身超时（55s）
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.jwt_ttl_hours", 72)
	v.SetDefault("auth.seed_user", "admin")
	v.SetDefault("auth.seed_pass", "admin123")
	v.SetDefault("auth.internal_key", "") // 为空则内网管理接口不注册
	// 配额与并发闸门（口径见 shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §2）
	v.SetDefault("quota.daily_tokens_per_user", 100000) // 每用户每自然日 10 万 token；0 = 不限
	v.SetDefault("quota.daily_tokens_global", 0)        // 全站日预算（熔断）；0 = 关闭
	v.SetDefault("quota.max_concurrent_per_user", 2)    // 同一用户同时进行中的请求数；0 = 不限
	v.SetDefault("quota.metric", "total")               // total | cached_weighted（预留，暂只实现 total）
}
