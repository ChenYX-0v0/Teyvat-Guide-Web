package dto

// AI 提示词内部只读接口（契约见 shared-docs/03-主服务对接说明.md §5）
// AI 服务侧解析见 ai-service/app/store/http_store.py：只取 sceneKey / content / version。

// AIPromptItem 一个场景当前生效的提示词
type AIPromptItem struct {
	SceneKey string `json:"sceneKey"`
	Content  string `json:"content"`
	Version  int    `json:"version"`
}

// AIPromptList 提示词列表（AI 服务按 ?scenes= 拉取；未命中的场景不出现）
type AIPromptList struct {
	Items []AIPromptItem `json:"items"`
}

// ---------- 后台维护（内网接口，见 shared-docs/03-主服务对接说明.md §5.2） ----------
//
// 后台只能"新增版本"与"启用 / 停用"：内容按版本不可变（改文案 = 新增一版），
// 回滚 = 把新版本 enabled 置 0，自动回落到上一启用版本。

// AIPromptAdminItem 后台列表项（含未启用版本；带完整内容，便于预览与"复制为新版本"）。
// 行数极少（人格场景 3~5 个 + 各自历史版本），因此不做摘要截断，与对话记录的量级不同。
type AIPromptAdminItem struct {
	ID       uint64 `json:"id"`
	SceneKey string `json:"sceneKey"`
	Version  int    `json:"version"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
	Content  string `json:"content"`
	// Served 该场景是否会被 AI 服务真正读取：不在白名单里的场景可以入库（提前准备），但当前不生效
	Served bool `json:"served"`
	// Effective 该版本是否就是该场景当前生效版本（启用且版本最高）
	Effective bool   `json:"effective"`
	CreatedAt string `json:"createdAt"` // RFC3339
	UpdatedAt string `json:"updatedAt"` // RFC3339
}

// AIPromptAdminList 后台列表
type AIPromptAdminList struct {
	Items []AIPromptAdminItem `json:"items"`
	// ServedScenes AI 服务当前会读取的场景（**顺序即拼接顺序**，供前端提示运营）
	ServedScenes []string `json:"servedScenes"`
}

// AIPromptCreateRequest 新增一版。
// version **由服务端分配**（max+1），不接受客户端指定——避免版本号冲突与覆盖历史。
type AIPromptCreateRequest struct {
	SceneKey string `json:"sceneKey" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Remark   string `json:"remark"`
	// Enabled 缺省为 true（新增即可能生效）；置 false 可"先入库不发"
	Enabled *bool `json:"enabled"`
}

// AIPromptEnabledRequest 启用 / 停用（版本行唯一可改的字段）
type AIPromptEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}
