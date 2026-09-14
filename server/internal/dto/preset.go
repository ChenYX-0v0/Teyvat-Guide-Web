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
