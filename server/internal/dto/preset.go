package dto

// PresetQAItem 预设问答条目（匿名用户在对话页看到的内容）
type PresetQAItem struct {
	ID       uint   `json:"id"`
	SceneKey string `json:"sceneKey"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Sort     int    `json:"sort"`
}

// PresetQAList 预设问答列表（`GET /api/preset-qa` 的 data）
type PresetQAList struct {
	Items []PresetQAItem `json:"items"`
}

// ---------- 后台维护（内网接口，见 docs/04-API接口文档.md §7.3） ----------

// PresetQAAdminItem 后台条目：比 C 端多 enabled / sort / 时间，便于运营判断
type PresetQAAdminItem struct {
	ID        uint   `json:"id"`
	SceneKey  string `json:"sceneKey"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Sort      int    `json:"sort"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"` // RFC3339
	UpdatedAt string `json:"updatedAt"` // RFC3339
}

// PresetQAAdminList 后台列表（含已下架条目——下架用 enabled=0，不做物理删除）
type PresetQAAdminList struct {
	Items []PresetQAAdminItem `json:"items"`
}

// PresetQASaveRequest 新建 / 更新请求（更新为整体覆盖这几个字段）
type PresetQASaveRequest struct {
	SceneKey string `json:"sceneKey"`
	Question string `json:"question" binding:"required"`
	Answer   string `json:"answer" binding:"required"`
	Sort     int    `json:"sort"`
	// Enabled 新建时缺省为 true；更新时缺省表示**保持原值**（便于只改文案）
	Enabled *bool `json:"enabled"`
}
