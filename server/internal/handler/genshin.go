package handler

import (
	"errors"
	"net/http"
	"regexp"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"
	"teyvat-guide/server/internal/service/enka"

	"github.com/gin-gonic/gin"
)

var uidPattern = regexp.MustCompile(`^\d{9}$`)

// PlayerHandler 玩家查询接口（Enka 数据源）
type PlayerHandler struct {
	svc *service.PlayerService
}

func NewPlayerHandler(svc *service.PlayerService) *PlayerHandler {
	return &PlayerHandler{svc: svc}
}

// Player GET /api/genshin/player?uid=197794747[&refresh=true]
// uid 省略时取当前登录用户绑定的 UID（未登录且未绑定则报参数错误）
// 默认优先返回本地快照；refresh=true 强制调 Enka 上游并更新快照
//
// ⚠️ 2026-10-12（用户口径 A）：**登录用户查询成功即自动绑定该 UID** ✓ ——
// 省掉"查到档案后再点一次绑定"那一步 ✓；查别的 UID 即**改绑**（代价用户已确认 ✓）。
// 绑定点刻意放在**档案取回来之后** ✓：查一个不存在 / 未公开的 UID 不会把账号绑过去 ✗。
// 匿名（无 token）行为**完全不变** ✓ —— 没有账号可绑 ✓。
func (h *PlayerHandler) Player(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		// ⚠️ 匿名请求（无 token，uid=0）不要白查一次库 ✗ —— 否则 `Binding(0)` 会打出一条
		// `SELECT * FROM users WHERE id = 0` + GORM 的 `record not found` 噪音日志 ✓
		// （2026-10-11 从 access 日志里发现的 ✓）。带上 token 时才去取绑定 ✓。
		if userID := c.GetUint("uid"); userID != 0 {
			uid, _, _ = h.svc.Binding(userID)
		}
	}
	if !uidPattern.MatchString(uid) {
		response.BadRequest(c, "UID 格式有误，应为 9 位数字（或先在账号信息页绑定 UID）")
		return
	}
	forceRefresh := c.Query("refresh") == "true"

	profile, err := h.svc.Query(c.Request.Context(), uid, forceRefresh)
	if err != nil {
		status, message := mapQueryError(err)
		if status != http.StatusNotFound {
			logger.L().Warnw("query player failed", "uid", uid, "err", err)
		}
		response.Fail(c, status, status, message)
		return
	}

	// 登录用户**查询成功即自动绑定**该 UID（2026-10-12 口径 A ✓）。
	// ⚠️ 顺序要紧：必须放在下面"已收录角色累积"**之前** ✓ —— 那段的门槛是"查的 UID == 已绑定的 UID"，
	// 先绑上，第一次查询才能顺带把该 UID 的角色池累积进来 ✓（否则第一次查既不绑也不累积 ✗）。
	// 绑定失败**不影响本次查询** ✓（查询是主流程、绑定是顺带 ✓）—— 只记日志 ✓。
	if userID := c.GetUint("uid"); userID != 0 {
		changed, err := h.svc.AutoBind(userID, uid)
		if err != nil {
			logger.L().Warnw("auto bind uid failed", "user", userID, "gameUid", uid, "err", err)
		} else if changed {
			logger.L().Infow("uid auto bound on query", "user", userID, "gameUid", uid)
		}
	}

	// 「已收录角色」累积（2026-10-11，**只增/只改不删** ✓）：
	// ⚠️ 只在**查询的 UID 正是当前用户绑定的 UID** 时累积 ✓ ——
	// 否则任何人都能刷任意 UID 把表撑大 ✗，也等于替"别人的号"存了数据 ✗。
	// 失败不影响查询（service 内部只记日志 ✓）。
	if boundUID, _, _ := h.svc.Binding(c.GetUint("uid")); boundUID != "" && boundUID == uid {
		h.svc.CollectCharacters(uid, profile.Characters)
	}

	response.OK(c, profile)
}

// Characters GET /api/genshin/characters?uid=197794747 —— 已收录角色（2026-10-11 新增）
//
// uid 省略时取当前登录用户绑定的 UID ✓。
//
// ⚠️ 2026-10-18 起这是**账号页的默认（也是唯一自动）入口** ✓，且是 **cache-aside** 语义：
//   - 库里有数据 ⇒ 直接返回，**零上游请求** ✓；
//   - 库里没数据（新绑定 / 换过库 / 快照被 purge）⇒ **自动回源 Enka 并把展柜入库** ✓ 后再返回，
//     不需要用户先点一次「同步」✗（把系统该做的事推给用户是错的 ✓）；
//   - `?refresh=true` ⇒ 跳过"有数据就不用回源"的判断，强制走一次上游 ✓（手动刷新入口 ✓）。
//
// 只有**本人绑定的 UID** 允许回源写库 ✓（与 `/player` 的采集门槛一致 ✓）；
// 其它 UID 只读库 ✓。回源失败不报错 ✓：返回库里已有的数据，前端给"重试"入口 ✓。
//
// `/player` 现在只剩一个用途：首次输入 UID 查询（要顺带完成自动绑定 ✓）。
//
// 与 `/player` 的区别（两个口径别混 ✗）：
//   - `/player`  = **当前展柜**：快照缓存（TTL 10 分钟 ✓），会随用户换展柜而整体变化 ✓；
//   - `/characters` = **历史累积的角色池**：换出展柜的角色**不会丢** ✓（只增不删 ✓），
//     但不在展柜里的那几张卡**停在最后采集时刻** ✓（可能过时 ✗ ⇒ 前端展示 lastSeenAt ✓）。
//
// 响应里额外带 `profile` / `syncedAt`（取自本地快照 ✓）：页面头部要昵称 / 等级 / 头像，
// 而这条链路不许调上游 ⇒ 从快照带一份 ✓；快照为空时前端显示"还没同步过，点同步" ✓。
func (h *PlayerHandler) Characters(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		// 同 Player：匿名请求不查绑定 ✓（避免 `WHERE id = 0` 的噪音查询与日志 ✓）
		if userID := c.GetUint("uid"); userID != 0 {
			uid, _, _ = h.svc.Binding(userID)
		}
	}
	if !uidPattern.MatchString(uid) {
		response.BadRequest(c, "UID 格式有误，应为 9 位数字（或先在账号信息页绑定 UID）")
		return
	}
	// cache-aside（2026-10-18）：有就返回；没有就回源 Enka 并入库 ✓ —— 用户不需要知道"要不要手动同步"。
	// 采集门槛与 `/player` 一致：只对**本人绑定的 UID** 回源写库 ✓（防刷、防替别人存数据 ✓）。
	boundUID, _, _ := h.svc.Binding(c.GetUint("uid"))
	allowCollect := boundUID != "" && boundUID == uid

	col, err := h.svc.CharactersWithFallback(c.Request.Context(), uid, c.Query("refresh") == "true", allowCollect)
	if err != nil {
		logger.L().Warnw("read collected characters failed", "uid", uid, "err", err)
		response.Internal(c, "读取已收录角色失败")
		return
	}
	response.OK(c, col)
}

// ---------- 账号 ↔ UID 绑定（需登录） ----------

type bindRequest struct {
	UID    string `json:"uid" binding:"required"`
	Server string `json:"server,omitempty"` // 空时默认 cn_gf01
}

// Bind POST /api/genshin/binding —— 绑定 UID（绑定前实查验证）
func (h *PlayerHandler) Bind(c *gin.Context) {
	userID := c.GetUint("uid")
	if userID == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "请先登录")
		return
	}
	var req bindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误")
		return
	}
	if !uidPattern.MatchString(req.UID) {
		response.BadRequest(c, "UID 格式有误，应为 9 位数字")
		return
	}
	server := req.Server
	if server == "" {
		server = service.DefaultGenshinServer // 与自动绑定共用一份默认值 ✓
	}
	if err := h.svc.BindUID(c.Request.Context(), userID, req.UID, server); err != nil {
		logger.L().Warnw("bind uid failed", "user", userID, "uid", req.UID, "err", err)
		status, message := mapQueryError(err)
		response.Fail(c, status, status, "绑定失败："+message)
		return
	}
	logger.L().Infow("uid bound", "user", userID, "uid", req.UID, "server", server)
	response.OK(c, gin.H{"uid": req.UID, "server": server})
}

// GetBinding GET /api/genshin/binding —— 查询当前用户的绑定
func (h *PlayerHandler) GetBinding(c *gin.Context) {
	userID := c.GetUint("uid")
	if userID == 0 {
		response.OK(c, gin.H{"bound": false})
		return
	}
	uid, server, err := h.svc.Binding(userID)
	if err != nil {
		logger.L().Warnw("read binding failed", "user", userID, "err", err)
		response.Internal(c, "读取绑定信息失败")
		return
	}
	if uid == "" {
		response.OK(c, gin.H{"bound": false})
		return
	}
	response.OK(c, gin.H{"bound": true, "uid": uid, "server": server})
}

// ContextStatus GET /api/genshin/context-status —— 聊天页用：派蒙现在能不能看到我的角色？
//
// 2026-10-12 新增：用户报"智能问答页面取不到绑定 UID 的角色信息"，排查发现
// 「未绑定 / 上游失败 / 展柜为空」三条路径对用户是同一个结果且**毫无提示** ✗ ⇒
// 聊天页进页面时调一次这个接口，把原因说清楚（未绑定就去绑 / 展柜空就换回展柜 ✓）。
// 未登录时返回 bound=false ✓（与 Player 一样不做 `WHERE id = 0` 的噪音查询 ✓）。
func (h *PlayerHandler) ContextStatus(c *gin.Context) {
	response.OK(c, h.svc.ContextStatus(c.GetUint("uid")))
}

// Unbind DELETE /api/genshin/binding —— 解除绑定
func (h *PlayerHandler) Unbind(c *gin.Context) {
	userID := c.GetUint("uid")
	if userID == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "请先登录")
		return
	}
	if err := h.svc.Unbind(userID); err != nil {
		logger.L().Warnw("unbind failed", "user", userID, "err", err)
		response.Internal(c, "解绑失败")
		return
	}
	response.OK(c, gin.H{"bound": false})
}

// mapQueryError 将上游错误映射为 HTTP 状态码与友好文案
func mapQueryError(err error) (int, string) {
	switch {
	case errors.Is(err, enka.ErrPlayerNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, enka.ErrGameMaintenance):
		return http.StatusServiceUnavailable, err.Error()
	case errors.Is(err, enka.ErrUpstreamLimited):
		return http.StatusServiceUnavailable, err.Error()
	default:
		return http.StatusBadGateway, "查询失败：" + err.Error()
	}
}

// Snapshot GET /api/genshin/player-snapshot?uid=xxx —— 读取最近一次查询落库的档案快照
func (h *PlayerHandler) Snapshot(c *gin.Context) {
	uid := c.Query("uid")
	if !uidPattern.MatchString(uid) {
		response.BadRequest(c, "UID 格式有误，应为 9 位数字")
		return
	}
	snapshot, err := h.svc.Snapshot(uid)
	if err != nil {
		logger.L().Warnw("read player snapshot failed", "uid", uid, "err", err)
		response.Internal(c, "读取快照失败")
		return
	}
	if snapshot == nil {
		response.Fail(c, 404, 404, "该 UID 尚无查询记录，请先调用 /api/genshin/player")
		return
	}
	response.OK(c, snapshot)
}
