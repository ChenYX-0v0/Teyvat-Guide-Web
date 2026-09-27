package router

import (
	"net"
	"net/url"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/handler"
	"teyvat-guide/server/internal/middleware"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/service"
	"teyvat-guide/server/internal/service/asset"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// allowOrigin 判定跨域来源是否放行。
// 回环地址（localhost / 127.0.0.1 / ::1）在任何模式下都放行；
// debug 模式额外放行局域网私有网段（192.168/16、10/8、172.16/12），便于手机等设备联调；
// release 模式保持严格白名单，只认回环地址（见 shared-docs/04 用户体系与鉴权边界设计）。
func allowOrigin(origin, mode string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	if mode != "debug" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

// New 注册路由并返回引擎
func New(cfg *config.Config) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.AccessLog())

	r.Use(cors.New(cors.Config{
		AllowOriginFunc:  func(origin string) bool { return allowOrigin(origin, cfg.Server.Mode) },
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 建表需先于业务服务初始化（种子账号依赖 users 表）
	if database.DB() != nil {
		autoMigrate()
	}

	health := handler.NewHealthHandler()
	authSvc := service.NewAuthService(cfg.Auth, database.DB())
	auth := handler.NewAuthHandler(authSvc)
	assets := asset.New(database.DB())
	playerSvc := service.NewPlayerService(database.DB())
	player := handler.NewPlayerHandler(playerSvc)
	// 配额与并发闸门：**只能在主服务侧按用户强制**（AI 服务只认一把 X-AI-Key，不区分用户）。
	// 口径见 shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §2。
	quotaSvc := service.NewQuotaService(cfg.Quota)
	// 对话服务依赖玩家服务：登录且绑定 UID 时注入玩家档案上下文（§6）
	chat := handler.NewChatHandler(service.NewChatService(cfg.AI, playerSvc, quotaSvc))
	// 预设问答（免登录）：匿名用户在对话页看到的内容
	preset := handler.NewPresetHandler(service.NewPresetQAService())
	// 埋点采集（shared-docs/07-前端埋点与使用统计.md）：只收"服务端看不见"的前端行为，
	// 匿名可上报但不带任何匿名标识（D1）；失败一律静默，绝不影响主流程
	usageSvc := service.NewUsageService(cfg.Usage)
	usage := handler.NewUsageHandler(usageSvc)
	// 后台「使用统计」看板（内网接口，契约见 shared-docs/07 §6）
	usageAdmin := handler.NewUsageAdminHandler(usageSvc)
	// 会话管理（C 端，需登录）：列表 / 明细 / 重命名
	chatSession := handler.NewChatSessionHandler(service.NewChatSessionService())
	assetHandler := handler.NewAssetHandler(assets, playerSvc)
	// 用户管理（内网）：封禁状态同时用于鉴权中间件（既有 Token 立即失效）
	userAdminSvc := service.NewUserAdminService()
	internalUser := handler.NewInternalUserHandler(userAdminSvc)
	// 对话记录查看（内网，只读）：契约见 admin/06-对话记录查看设计.md
	internalChat := handler.NewInternalChatHandler(service.NewChatAdminService())
	// AI 提示词（内网，只读）：供 AI 派蒙服务拉取，契约见 shared-docs/03 主服务对接说明 §5
	internalAI := handler.NewInternalAIHandler(service.NewAIPromptService())
	// AI 提示词维护（内网，读写）：新增版本 / 启用与回滚，后台页面「提示词管理」。
	// 只提供"新增版本 + 启停"——内容按版本不可变，回滚不靠改内容（契约见 shared-docs/03 §5.2）
	internalAIAdmin := handler.NewInternalAIPromptAdminHandler(service.NewAIPromptAdminService())
	checkBanned := middleware.UserStatusChecker(userAdminSvc.IsBanned)
	if cfg.Auth.AdminKey == "" {
		logger.L().Warnw("auth.admin_key 未配置，管理接口回落使用 auth.jwt_secret；" +
			"建议为外部系统单独下发管理密钥（TEYVAT_AUTH_ADMIN_KEY）")
	}

	api := r.Group("/api")
	{
		api.GET("/health", health.Health)
		// 预设问答：**无需登录**——匿名用户在对话页看到的内容，不调模型、不消耗 token
		// （匿名禁用 AI 后的替代体验，见 shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §1）
		api.GET("/preset-qa", preset.List)

		// 埋点上报：**匿名可访问**（不带任何匿名标识，D1 定案）；带 Token 时记 user_id。
		// 契约见 shared-docs/07 §4.1 —— 任何情况都返回 200（accepted 只用于观测）
		api.POST("/events", middleware.JWTAuthOptional(cfg.Auth, checkBanned), usage.Ingest)

		// 对话：**强制登录**（2026-09-14 变更）。匿名不再具备 AI 对话能力——
		// 额度无归属、不可审计，也没有任何配额手段；/api/genshin/* 等只读接口仍匿名可用。
		api.POST("/chat", middleware.JWTAuth(cfg.Auth, checkBanned), chat.Chat)

		// 会话管理（需登录）：列表 / 明细 / 重命名 / 删除（软删，C 端不可见、后台可查）。
		// 未登录一律 401——匿名没有"我的会话"这个概念，前端也不为匿名做持久化（口径见 docs/04 §2.4）。
		sessions := api.Group("/sessions", middleware.JWTAuth(cfg.Auth, checkBanned))
		{
			sessions.GET("", chatSession.List)
			sessions.GET("/:sessionId/messages", chatSession.Messages)
			sessions.PATCH("/:sessionId", chatSession.Rename)
			sessions.DELETE("/:sessionId", chatSession.Delete)
		}

		// 玩家查询：可选鉴权（带 Token 时 uid 可省略，自动用绑定 UID）
		genshin := api.Group("/genshin", middleware.JWTAuthOptional(cfg.Auth, checkBanned))
		{
			genshin.GET("/player", player.Player)
			genshin.GET("/player-snapshot", player.Snapshot)
			genshin.POST("/binding", player.Bind)
			genshin.GET("/binding", player.GetBinding)
			genshin.DELETE("/binding", player.Unbind)
		}

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", auth.Register)
			authGroup.POST("/login", auth.Login)
			authGroup.GET("/me", middleware.JWTAuth(cfg.Auth, checkBanned), auth.Me)
			// 用户须知（免责声明）同意留痕：版本号由主服务校验（只接受 CurrentTermsVersion），
			// 前端"强制阅读 5 秒 → 同意"后调用；服务端存列，作为"已告知"的证据（见 service/terms.go）。
			authGroup.POST("/terms", middleware.JWTAuth(cfg.Auth, checkBanned), auth.AcceptTerms)
		}

		// 管理接口（X-Admin-Key 保护）：资产映射导入 / 缺漏 / 重载
		admin := api.Group("/admin", middleware.AdminKey(cfg.Auth))
		{
			admin.POST("/assets/import", assetHandler.Import)
			admin.GET("/assets/missing", assetHandler.Missing)
			admin.POST("/assets/reload", assetHandler.Reload)
			admin.POST("/assets/purge-snapshots", assetHandler.PurgeSnapshots)
		}
	}

	// 内网管理接口（X-Internal-Key 保护，契约路径不带 /api 前缀）：供后台运维系统调用
	// internal_key 未配置时不注册（安全默认：不暴露无鉴权接口）
	if cfg.Auth.InternalKey == "" {
		logger.L().Warnw("auth.internal_key 未配置，内网接口未注册",
			"hint", "配置 TEYVAT_AUTH_INTERNAL_KEY 后重启启用 /internal/v1/admin/* 与 /internal/v1/ai/prompts")
	} else {
		internal := r.Group("/internal/v1/admin", middleware.InternalKeyAuth(cfg.Auth))
		{
			internal.GET("/users", internalUser.ListUsers)
			internal.POST("/users/status", internalUser.SetUserStatus)
			// 后台代建账号（邀请制下的唯一开户入口）：公开注册关闭时（auth.allow_register=false）
			// 账号只能在这里创建——`CreateUser` 刻意不受该开关限制
			internal.POST("/users", internalUser.CreateUser)
			// 使用统计看板（只读）：概览 / 功能排行榜 / 趋势 / 漏斗 / 导出
			internal.GET("/usage/overview", usageAdmin.Overview)
			internal.GET("/usage/events", usageAdmin.Events)
			internal.GET("/usage/series", usageAdmin.Series)
			internal.GET("/usage/funnel", usageAdmin.Funnel)
			internal.GET("/usage/export", usageAdmin.Export)
			// 对话记录查看（只读，无修改/删除能力）
			internal.GET("/chat/sessions", internalChat.ListChatSessions)
			internal.GET("/chat/sessions/:sessionId/messages", internalChat.ListChatMessages)
			// 预设问答维护（后台可运营：新建 / 编辑 / 上下架；**不做物理删除**，
			// 下架用 enabled=0 —— 与"预设问答要能随时改、随时撤"的运营诉求一致）
			internal.GET("/preset-qa", preset.AdminList)
			internal.POST("/preset-qa", preset.Create)
			internal.PUT("/preset-qa/:id", preset.Update)
			// 提示词维护（后台可运营：新增版本 / 启用回滚；**不提供改内容**，
			// 改文案一律"新增一版"、老版本留作回滚——见 shared-docs/03 §5.2）
			internal.GET("/ai-prompts", internalAIAdmin.AdminList)
			internal.POST("/ai-prompts", internalAIAdmin.Create)
			internal.PUT("/ai-prompts/:id", internalAIAdmin.SetEnabled)
		}

		// AI 提示词只读接口：AI 派蒙服务在 PROMPT_STORE_MODE=http 时按场景拉取
		// （路径不带 /admin：它是"AI 服务 ↔ 主服务"的对接面，不是后台运维面）
		ai := r.Group("/internal/v1/ai", middleware.InternalKeyAuth(cfg.Auth))
		{
			ai.GET("/prompts", internalAI.GetAIPrompts)
		}
	}

	return r
}

// autoMigrate 注册需要自动建表的模型
func autoMigrate() {
	if err := database.DB().AutoMigrate(
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.PresetQA{},
		&model.PlayerProfileSnapshot{},
		&model.User{},
		&model.AIPrompt{},
		&model.UsageEvent{}, // 埋点明细（shared-docs/07）
		&model.UsageDaily{}, // 埋点按天聚合（看板读它）
		&asset.Character{},
		&asset.Weapon{},
		&asset.RelicSet{},
		&asset.RelicSetEffect{},
		&asset.TextMapEntry{},
		&asset.PropMap{},
		&asset.FightProp{},
		&asset.RelicSlot{},
		&asset.Affix{},
		&asset.CharacterStatWeights{},
		&asset.CharacterRelicWeight{},     // 圣遗物评分权重（V3，2026-09-27，见 ai/08）
		&asset.CharacterRelicWeightRule{}, // 评分权重·角色级覆盖规则
		&asset.Missing{},
	); err != nil {
		logger.L().Errorw("auto migrate failed", "err", err)
	}
}
