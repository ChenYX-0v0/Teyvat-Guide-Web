package router

import (
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

// New 注册路由并返回引擎
func New(cfg *config.Config) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.AccessLog())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
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
	// 对话服务依赖玩家服务：登录且绑定 UID 时注入玩家档案上下文（§6）
	chat := handler.NewChatHandler(service.NewChatService(cfg.AI, playerSvc))
	assetHandler := handler.NewAssetHandler(assets, playerSvc)
	// 用户管理（内网）：封禁状态同时用于鉴权中间件（既有 Token 立即失效）
	userAdminSvc := service.NewUserAdminService()
	internalUser := handler.NewInternalUserHandler(userAdminSvc)
	// 对话记录查看（内网，只读）：契约见 admin/06-对话记录查看设计.md
	internalChat := handler.NewInternalChatHandler(service.NewChatAdminService())
	checkBanned := middleware.UserStatusChecker(userAdminSvc.IsBanned)
	if cfg.Auth.AdminKey == "" {
		logger.L().Warnw("auth.admin_key 未配置，管理接口回落使用 auth.jwt_secret；" +
			"建议为外部系统单独下发管理密钥（TEYVAT_AUTH_ADMIN_KEY）")
	}

	api := r.Group("/api")
	{
		api.GET("/health", health.Health)
		// 对话：可选鉴权（带 Token 时注入用户身份 → 追加玩家上下文；匿名照常可用）
		api.POST("/chat", middleware.JWTAuthOptional(cfg.Auth, checkBanned), chat.Chat)

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
		logger.L().Warnw("auth.internal_key 未配置，内网管理接口未注册",
			"hint", "配置 TEYVAT_AUTH_INTERNAL_KEY 后重启启用 /internal/v1/admin/*")
	} else {
		internal := r.Group("/internal/v1/admin", middleware.InternalKeyAuth(cfg.Auth))
		{
			internal.GET("/users", internalUser.ListUsers)
			internal.POST("/users/status", internalUser.SetUserStatus)
			// 对话记录查看（只读，无修改/删除能力）
			internal.GET("/chat/sessions", internalChat.ListChatSessions)
			internal.GET("/chat/sessions/:sessionId/messages", internalChat.ListChatMessages)
		}
	}

	return r
}

// autoMigrate 注册需要自动建表的模型
func autoMigrate() {
	if err := database.DB().AutoMigrate(
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.PlayerProfileSnapshot{},
		&model.User{},
		&asset.Character{},
		&asset.Weapon{},
		&asset.RelicSet{},
		&asset.TextMapEntry{},
		&asset.PropMap{},
		&asset.FightProp{},
		&asset.RelicSlot{},
		&asset.Affix{},
		&asset.CharacterStatWeights{},
		&asset.Missing{},
	); err != nil {
		logger.L().Errorw("auto migrate failed", "err", err)
	}
}
