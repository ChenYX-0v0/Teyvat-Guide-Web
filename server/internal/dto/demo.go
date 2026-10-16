package dto

import "encoding/json"

// ---------- 游客演示模式（`GET /api/demo/showcase`，**无需登录**） ----------
//
// 口径：演示页**不连 AI 服务**，全部内容来自库里预置的 demo_profiles / demo_characters ✓。
// 这条链路不调模型、不消耗 token、不落库会话 ✓。

// DemoProfileInfo 模拟旅行者账号头信息
type DemoProfileInfo struct {
	Nickname     string `json:"nickname"`
	Signature    string `json:"signature"`
	Level        int    `json:"level"`
	WorldLevel   int    `json:"worldLevel"`
	Achievements int    `json:"achievements"`
	// Abyss 深渊概要；演示数据没配时为 nil（前端整块不渲染 ✓）
	Abyss *AbyssSummary `json:"abyss,omitempty"`
}

// DemoCharacterItem 演示角色 = 定位标签 + **与真实档案同构的角色卡** + 预置分析 ✓
//
// Card 用 json.RawMessage 透传：卡结构直接由 dto.CharacterCard 序列化而来，
// 这里不再重复声明 20 个字段（避免两处定义漂移 ✗）✓。
type DemoCharacterItem struct {
	CharKey  string          `json:"charKey"`
	Sort     int             `json:"sort"`
	Name     string          `json:"name"`
	Element  string          `json:"element"`
	Rarity   int             `json:"rarity"`
	Role     string          `json:"role"`
	Card     json.RawMessage `json:"card"`
	Analysis string          `json:"analysis"`
}

// DemoShowcase 演示页一次性取回的数据（账号头 + 4 位角色）✓
type DemoShowcase struct {
	Profile    *DemoProfileInfo    `json:"profile"`
	Characters []DemoCharacterItem `json:"characters"`
	// Notice 前端要显示的"演示内容"提示语（写死在代码里反而不好改，放库里可运营）✓
	Notice string `json:"notice"`
}
