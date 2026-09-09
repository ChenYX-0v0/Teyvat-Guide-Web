package enka

// Enka.Network 客户端：免认证查询 + 进程内缓存
// 错误映射：400→玩家不存在/未公开；424→游戏维护；429→上游限流

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const baseURL = "https://enka.network"

// 上游错误分类（service 层映射为 HTTP 状态码与友好文案）
var (
	ErrPlayerNotFound = errors.New("玩家不存在，或未在游戏内公开详细信息")
	ErrGameMaintenance = errors.New("游戏维护中，请稍后再试")
	ErrUpstreamLimited = errors.New("上游查询过于频繁，请稍后再试")
)

type cacheEntry struct {
	profile *Profile
	expires time.Time
}

type Client struct {
	http *http.Client

	mu       sync.RWMutex
	cache    map[string]cacheEntry
	negative map[string]time.Time

	ttl         time.Duration
	negativeTTL time.Duration
}

func NewClient() *Client {
	return &Client{
		http:        &http.Client{Timeout: 15 * time.Second},
		cache:       make(map[string]cacheEntry),
		negative:    make(map[string]time.Time),
		ttl:         60 * time.Second,   // 与上游 ttl 对齐
		negativeTTL: 5 * time.Minute,    // 负缓存防扫号穿透
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

	profile, err := c.fetch(ctx, uid)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		if errors.Is(err, ErrPlayerNotFound) {
			c.negative[uid] = time.Now().Add(c.negativeTTL)
		}
		return nil, err
	}
	ttl := c.ttl
	if profile.TTL > 0 && profile.TTL < 60 {
		ttl = time.Duration(profile.TTL) * time.Second
	}
	c.cache[uid] = cacheEntry{profile: profile, expires: time.Now().Add(ttl)}
	return profile, nil
}

func (c *Client) fetch(ctx context.Context, uid string) (*Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/uid/"+uid+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "teyvat-guide/0.1")

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
	case http.StatusBadRequest:
		return nil, ErrPlayerNotFound
	case http.StatusFailedDependency: // 424 游戏维护
		return nil, ErrGameMaintenance
	case http.StatusTooManyRequests:
		return nil, ErrUpstreamLimited
	default:
		return nil, fmt.Errorf("Enka HTTP %d: %s", resp.StatusCode, truncate(raw, 200))
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
