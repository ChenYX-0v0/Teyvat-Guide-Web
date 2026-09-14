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
