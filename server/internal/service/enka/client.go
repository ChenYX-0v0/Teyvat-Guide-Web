package enka

// Enka.Network 客户端：免认证查询 + 进程内缓存
// 错误映射：400（UID 非法）/ 404（玩家不存在或未公开）→ ErrPlayerNotFound；
// 424→游戏维护；429→上游限流；其余→通用上游错误（**细节只进日志，不回传上游原文**）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"teyvat-guide/server/internal/pkg/logger"
)

const baseURL = "https://enka.network"

// 上游错误分类（service 层映射为 HTTP 状态码与友好文案）
var (
	ErrPlayerNotFound  = errors.New("玩家不存在，或未在游戏内公开详细信息")
	ErrGameMaintenance = errors.New("游戏维护中，请稍后再试")
	ErrUpstreamLimited = errors.New("上游查询过于频繁，请稍后再试")
)

type cacheEntry struct {
	profile *Profile
	expires time.Time
}

// flight 同一 UID 的并发请求合并（singleflight），避免缓存击穿打爆上游
type flight struct {
	done    chan struct{}
	profile *Profile
	err     error
}

type Client struct {
	http *http.Client

	mu       sync.RWMutex
	cache    map[string]cacheEntry
	negative map[string]time.Time
	flights  map[string]*flight

	ttl         time.Duration
	negativeTTL time.Duration
}

func NewClient() *Client {
	return &Client{
		http:        &http.Client{Timeout: 15 * time.Second},
		cache:       make(map[string]cacheEntry),
		negative:    make(map[string]time.Time),
		flights:     make(map[string]*flight),
		ttl:         60 * time.Second, // 与上游 ttl 对齐
		negativeTTL: 5 * time.Minute,  // 负缓存防扫号穿透
	}
}

// FetchProfile 拉取玩家数据（带缓存）
func (c *Client) FetchProfile(ctx context.Context, uid string) (*Profile, error) {
	// 负缓存检查
	c.mu.RLock()
	if nexp, ok := c.negative[uid]; ok && time.Now().Before(nexp) {
		c.mu.RUnlock()
		return nil, ErrPlayerNotFound
	}
	// 正缓存命中
	if hit, ok := c.cache[uid]; ok && time.Now().Before(hit.expires) {
		c.mu.RUnlock()
		return hit.profile, nil
	}
	c.mu.RUnlock()

	// 同一 UID 已有请求在飞行中：等待其结果（singleflight）
	c.mu.Lock()
	if f, ok := c.flights[uid]; ok {
		c.mu.Unlock()
		<-f.done
		return f.profile, f.err
	}
	f := &flight{done: make(chan struct{})}
	c.flights[uid] = f
	c.mu.Unlock()

	profile, err := c.fetch(ctx, uid)

	c.mu.Lock()
	delete(c.flights, uid)
	if err != nil {
		if errors.Is(err, ErrPlayerNotFound) {
			c.negative[uid] = time.Now().Add(c.negativeTTL)
		}
		f.err = err
	} else {
		ttl := c.ttl
		if profile.TTL > 0 && profile.TTL < 60 {
			ttl = time.Duration(profile.TTL) * time.Second
		}
		c.cache[uid] = cacheEntry{profile: profile, expires: time.Now().Add(ttl)}
		f.profile = profile
	}
	close(f.done)
	c.mu.Unlock()

	return profile, err
}

func (c *Client) fetch(ctx context.Context, uid string) (*Profile, error) {
	// 路径末尾**不加**斜杠：Enka 对带斜杠的路径会先回 308 跳到无斜杠版本（2026-09-21 实测），
	// 多一个来回。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/uid/"+uid, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "teyvat-guide/0.1")
	// ⚠️ 必须显式声明 Accept：不带此头时，Enka 对「不存在的玩家」返回的是 **HTML 错误页**
	// （`<title>This player does not exist.</title>`）而不是 JSON，会被当成响应体读进来，
	// 一旦状态码没被下面的 switch 覆盖，就会把这段 HTML 原样拼进给终端用户的错误信息
	// （2026-09-21 线上实测踩到）。带上后 404 稳定返回 {"message":"..."}，便于判别。
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Enka 失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// 正常
	case http.StatusBadRequest, http.StatusNotFound:
		// 400 = UID 格式非法；404 = 玩家不存在，或未在游戏内开启「展示角色详情」。
		// ⚠️ Enka 用的就是 **404**（不是 400）。此前只判 400，导致 404 掉进 default 分支，
		// 把上游的 HTML 错误页透给了终端用户（2026-09-21 实测踩到）。
		return nil, ErrPlayerNotFound
	case http.StatusFailedDependency: // 424 游戏维护
		return nil, ErrGameMaintenance
	case http.StatusTooManyRequests:
		return nil, ErrUpstreamLimited
	default:
		// 上游异常：原始响应（可能是 HTML）只记日志，**不回传给终端用户**
		logger.L().Warnw("enka unexpected response",
			"uid", uid, "status", resp.StatusCode, "body", truncate(raw, 200))
		return nil, fmt.Errorf("上游返回异常状态 %d，请稍后再试", resp.StatusCode)
	}

	var out Profile
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if out.UID == "" {
		out.UID = uid
	}
	return &out, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
