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
	chat := handler.NewChatHandler(service.NewChatService(cfg.AI))
	authSvc := service.NewAuthService(cfg.Auth, database.DB())
	auth := handler.NewAuthHandler(authSvc)
	assets := asset.New(database.DB())
	player := handler.NewPlayerHandler(service.NewPlayerService(database.DB()))
	assetHandler := handler.NewAssetHandler(assets)

	api := r.Group("/api")
	{
		api.GET("/health", health.Health)
		api.POST("/chat", chat.Chat)
		api.GET("/genshin/player", player.Player)
		api.GET("/genshin/player-snapshot", player.Snapshot)

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", auth.Register)
			authGroup.POST("/login", auth.Login)
			authGroup.GET("/me", middleware.JWTAuth(cfg.Auth), auth.Me)
		}

		// 管理接口（X-Admin-Key 保护）：资产映射导入 / 缺漏 / 重载
		admin := api.Group("/admin", middleware.AdminKey(cfg.Auth))
		{
			admin.POST("/assets/import", assetHandler.Import)
			admin.GET("/assets/missing", assetHandler.Missing)
			admin.POST("/assets/reload", assetHandler.Reload)
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
		&asset.Affix{},
		&asset.Missing{},
	); err != nil {
		logger.L().Errorw("auto migrate failed", "err", err)
	}
}
