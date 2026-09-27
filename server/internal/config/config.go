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
	Usage    UsageConfig    `mapstructure:"usage"`
	SMS      SMSConfig      `mapstructure:"sms"`
}

// SMSConfig 短信验证码（阿里云号码认证服务 Dypnsapi；2026-09-27 新增，用于**手机号注册**）
//
// 口径：注册 = 手机号 + 短信验证码 + 设置密码（登录仍用手机号/密码，见 service/auth.go）。
// 验证码由阿里云生成与校验（SendSmsVerifyCode / CheckSmsVerifyCode）⇒ 我们**不落库验证码** ✓。
// ⚠️ 默认 **Enabled=false**：没配好签名/密钥时绝不能误发短信（发一条就是钱 ✗）。
type SMSConfig struct {
	// Enabled 总开关；false 时 /api/auth/sms/send 直接拒绝、手机号注册不可用
	Enabled bool `mapstructure:"enabled"`
	// Provider 服务商；当前只实现 aliyun（留字段便于以后换）
	Provider string `mapstructure:"provider"`
	// SignName 短信签名（阿里云控制台已审核的签名，如 恒创联众）
	SignName string `mapstructure:"sign_name"`
	// TemplateCode 模板号（模板里用 `##code##` 占位，见 TemplateParam）
	TemplateCode string `mapstructure:"template_code"`
	// Endpoint 服务接入点（Dypnsapi 固定为 dypnsapi.aliyuncs.com）
	Endpoint string `mapstructure:"endpoint"`
	// AccessKeyID / AccessKeySecret 显式 AK；**都留空则走"无 AK"凭据链** ✓
	// （ECS 实例 RAM 角色 / 环境变量 ALIBABA_CLOUD_ACCESS_KEY_ID 等，见部署说明）
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	// Mock 本地/联调用：自建验证码 + 进程内校验，不真发短信（只在日志里打验证码 ✓）
	Mock bool `mapstructure:"mock"`
	// CodeTTLMinutes 验证码有效期（分钟）；同时作为短信文案里的 min 与阿里云 ValidTime
	CodeTTLMinutes int `mapstructure:"code_ttl_minutes"`
	// SendIntervalSeconds 同一手机号两次发送的最小间隔（秒）
	SendIntervalSeconds int `mapstructure:"send_interval_seconds"`
	// DailyLimitPerPhone 同一手机号每自然日发送上限（防单号被刷）
	DailyLimitPerPhone int `mapstructure:"daily_limit_per_phone"`
	// HourlyLimitPerIP 同一来源 IP 每小时发送上限（防换号刷；IP 只存进程内，不落库）
	HourlyLimitPerIP int `mapstructure:"hourly_limit_per_ip"`
}

// UsageConfig 埋点采集（口径真源：shared-docs/07-前端埋点与使用统计.md）
//
// D1（2026-09-26 定）：**不采集匿名标识** —— 匿名行为只有次数，没有 UV。
// D2（2026-09-26 定）：**采集停留时长**（page_leave 的 dwell_ms，上报前上下截断）。
type UsageConfig struct {
	// Enabled 总开关；false 时 /api/events 一律 accepted=0（不落库、不报错）
	Enabled bool `mapstructure:"enabled"`
	// RetentionDays 明细保留天数；0 = 不清理（聚合表永久保留）
	RetentionDays int `mapstructure:"retention_days"`
	// MaxBatch 单请求最多条数（与 DTO 的 max 保持一致）
	MaxBatch int `mapstructure:"max_batch"`
	// MaxPropsBytes 单条 props 序列化后的上限，超限丢弃
	MaxPropsBytes int `mapstructure:"max_props_bytes"`
	// IPPerMinute / UserPerMinute 限流：每分钟上限，超限静默丢弃（0 = 不限）
	IPPerMinute   int `mapstructure:"ip_per_minute"`
	UserPerMinute int `mapstructure:"user_per_minute"`
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
	// MaxRequestsPerMinute 同一用户每分钟 AI 请求上限（60 秒滑窗）；0 = 不限制
	// 定位：日额度按 token 计，拦不住"反复刷越界问题"（那些请求几乎不花 token ✗）
	// 默认 20（setDefaults 注册）；正常用户每分钟 1~3 问，不会碰到 ✓
	MaxRequestsPerMinute int `mapstructure:"max_requests_per_minute"`
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
	// AllowRegister 是否开放公开注册。false = 不公开/邀请制：`POST /api/auth/register` 直接 403，
	// 账号只能在后台「用户管理 → 新增用户」里由管理员创建（内网接口）。
	// 口径见 shared-docs/04-用户体系与鉴权边界设计.md §2.3 与 web/11 §7.6。
	AllowRegister bool   `mapstructure:"allow_register"`
	SeedUser      string `mapstructure:"seed_user"` // 首次启动种子账号（用户表为空时自动创建）
	SeedPass      string `mapstructure:"seed_pass"`
	AdminKey      string `mapstructure:"admin_key"`    // 管理接口密钥（X-Admin-Key）；为空时回落 jwt_secret
	InternalKey   string `mapstructure:"internal_key"` // 内网管理接口密钥（X-Internal-Key）；为空则内网接口不注册（安全默认）
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
	// ⚠️ 这一行不是"为了有个默认值"，而是必须：viper 的 Unmarshal 只回填**已知键**
	// （config 文件里出现过的，或 SetDefault 注册过的）；只在 AutomaticEnv 里存在的键，
	// Unmarshal **看不见**。容器里没有 config.yaml，少这一行 → TEYVAT_AI_API_KEY 静默失效
	// → 主服务调 AI 服务不带 X-AI-Key → 401「缺少 X-AI-Key」（2026-09-19 容器实测踩到）。
	// 新增任何配置项时，务必同样补一行 SetDefault。
	v.SetDefault("ai.api_key", "")
	v.SetDefault("ai.base_url", "https://api.openai.com/v1")
	// ⚠️ 默认留空 = 让 AI 服务用它自己的默认模型（当前 deepseek-flash）。
	// 这里原本是模板遗留的 "gpt-4o-mini"：本地有 config.yaml 时无感；但容器里没有配置文件，
	// 这个默认值会被传给 AI 服务 → 上游 DeepSeek 直接 400
	// 「The supported API model names are deepseek-flash, deepseek-v4-pro, but you passed gpt-4o-mini」
	// （2026-09-19 容器实测踩到）。要换模型请在 .env 显式设 TEYVAT_AI_MODEL。
	v.SetDefault("ai.model", "")
	// 人格提示词归 AI 服务（存 ai_prompts 表，可按版本热更新），这里默认为空 = 不下发补充指令。
	// 注意：一旦填了值，它会作为动态区拼在派蒙人格**之后**（persona.py 的 request_system），
	// 等于第二套人格，与 ai_prompts 里的派蒙设定冲突——要加"每请求补充指令"请入库成新场景。
	v.SetDefault("ai.system", "")
	v.SetDefault("ai.timeout_seconds", 65) // 必须大于 AI 服务自身超时（55s）
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.jwt_ttl_hours", 72)
	// 同 ai.api_key：不注册默认值 → 容器里 TEYVAT_AUTH_ADMIN_KEY 失效 →
	// AdminAPIKey() 回落到 jwt_secret → 后台带自己的 admin_key 调管理接口反而 401。
	v.SetDefault("auth.admin_key", "")
	// 默认沿用"开放注册"的通用行为；**本项目当前部署口径是在 config.yaml 里置 false（邀请制）**
	v.SetDefault("auth.allow_register", true)
	v.SetDefault("auth.seed_user", "admin")
	v.SetDefault("auth.seed_pass", "admin123")
	v.SetDefault("auth.internal_key", "") // 为空则内网管理接口不注册
	// 配额与并发闸门（口径见 shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §2）
	v.SetDefault("quota.daily_tokens_per_user", 100000) // 每用户每自然日 10 万 token；0 = 不限
	v.SetDefault("quota.daily_tokens_global", 0)        // 全站日预算（熔断）；0 = 关闭
	v.SetDefault("quota.max_concurrent_per_user", 2)    // 同一用户同时进行中的请求数；0 = 不限
	// 分钟级滑窗限速（L4）：拦住"短时间反复刷"（越界提问几乎不花 token，日额度拦不住 ✗）
	v.SetDefault("quota.max_requests_per_minute", 20)  // 每分钟请求数；0 = 不限
	v.SetDefault("quota.whitelist_user_ids", []uint{}) // 同理：注册默认值，键才会被 Unmarshal 认到
	v.SetDefault("quota.metric", "total")              // total | cached_weighted（预留，暂只实现 total）
	// 埋点（shared-docs/07）。⚠️ 同 ai.api_key：每个新配置项都必须 SetDefault，
	// 否则容器里（无 config.yaml）该项会被 Unmarshal 静默忽略。
	v.SetDefault("usage.enabled", true)
	v.SetDefault("usage.retention_days", 90)    // 明细保留天数；0 = 不清理
	v.SetDefault("usage.max_batch", 20)         // 单请求最多条数
	v.SetDefault("usage.max_props_bytes", 2048) // 单条 props 上限，超限丢弃
	v.SetDefault("usage.ip_per_minute", 60)     // 每 IP 每分钟上限，超限静默丢弃
	v.SetDefault("usage.user_per_minute", 300)  // 每登录用户每分钟上限

	// 短信验证码（手机号注册，口径见 shared-docs/04 §2 与 web/04 §3）
	// ⚠️ 默认 **关闭**：签名/密钥没配好时不应误发短信（发一条就是钱 ✗）。开启只需
	// `TEYVAT_SMS_ENABLED=true` + 密钥（无 AK 方式则给 ECS 绑 RAM 角色即可 ✓）。
	v.SetDefault("sms.enabled", false)
	v.SetDefault("sms.provider", "aliyun")
	v.SetDefault("sms.sign_name", "恒创联众")       // 阿里云已审核签名（用户在控制台申请的）
	v.SetDefault("sms.template_code", "100001") // 模板：您好，验证码 ##code##，##min## 分钟内有效
	v.SetDefault("sms.endpoint", "dypnsapi.aliyuncs.com")
	v.SetDefault("sms.access_key_id", "")     // 留空 = 走无 AK 凭据链（ECS RAM 角色）✓
	v.SetDefault("sms.access_key_secret", "") // 同上；两者都空才走凭据链
	v.SetDefault("sms.mock", false)           // true = 不真发短信（本地/联调）
	v.SetDefault("sms.code_ttl_minutes", 5)   // 验证码 5 分钟有效（与短信文案一致 ✓）
	v.SetDefault("sms.send_interval_seconds", 60)
	v.SetDefault("sms.daily_limit_per_phone", 10)
	v.SetDefault("sms.hourly_limit_per_ip", 20)
}
