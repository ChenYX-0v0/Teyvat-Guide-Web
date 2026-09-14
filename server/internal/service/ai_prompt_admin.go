package service

// AI 提示词的**后台维护**（新增版本 / 启用回滚）；读取侧见 ai_prompt.go。
//
// 契约：shared-docs/03-主服务对接说明.md §5.2
//
// 三条口径：
// 1. **内容不可变**：改文案 = 新增一版（服务端分配 version = max+1），老版本永不修改，
//    这样"线上是哪一版、什么时候换的"永远可追；
// 2. **生效 = 启用且版本最高**：新增一版并启用即成为生效版本，AI 服务最迟 60s（TTL）拉到；
// 3. **回滚 = 把新版本 enabled 置 0**：自动回落到该场景下一个启用版本，全程不改内容。

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
)

const (
	aiPromptContentMaxRunes = 20000 // 人格类提示词的合理上限（TEXT 列本身可放 64KB）
	aiPromptRemarkMaxRunes  = 200   // 表列 VARCHAR(255)
)

var (
	// ErrAIPromptNotFound 提示词版本不存在（Handler 映射 404）
	ErrAIPromptNotFound = errors.New("提示词版本不存在")

	// ErrAIPromptInvalid 参数不合法（Handler 映射 400）
	ErrAIPromptInvalid = errors.New("提示词参数有误")
)

// aiPromptScenePattern 场景键：小写字母开头，后接小写字母 / 数字 / 下划线，最长 64
var aiPromptScenePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// aiPromptServedScenes AI 服务当前会真正读取的场景，**顺序即拼接顺序**（稳定段 → 动态段）。
// 必须与 ai-service/app/chains/persona.py 的 STABLE_SCENES 保持一致：
// 不在白名单里的场景可以入库（提前准备 / 留档），但当前不会被读，管理页据此提示运营。
var aiPromptServedScenes = []string{"paimon_base", "paimon_rules", "paimon_fewshot"}

// AIPromptAdminService 提示词后台维护
type AIPromptAdminService struct {
	repo *repository.AIPromptRepository
}

func NewAIPromptAdminService() *AIPromptAdminService {
	return &AIPromptAdminService{repo: repository.NewAIPromptRepository()}
}

// ListAll 全部版本（含未启用），并标出"每个场景当前生效版本"
func (s *AIPromptAdminService) ListAll() (*dto.AIPromptAdminList, error) {
	rows, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}

	// rows 已按 (scene_key asc, version desc) 排序：每个场景第一条 enabled 的行即生效版本
	items := make([]dto.AIPromptAdminItem, 0, len(rows))
	seenEffective := make(map[string]bool, len(rows))
	for _, row := range rows {
		effective := row.Enabled && !seenEffective[row.SceneKey]
		if effective {
			seenEffective[row.SceneKey] = true
		}
		items = append(items, toAdminPromptItem(row, isServedPromptScene(row.SceneKey), effective))
	}

	return &dto.AIPromptAdminList{
		Items:        items,
		ServedScenes: append([]string(nil), aiPromptServedScenes...),
	}, nil
}

// Create 新增一版：版本号由服务端分配（当前最大版本 + 1），不使用客户端传的版本号
func (s *AIPromptAdminService) Create(req dto.AIPromptCreateRequest) (*dto.AIPromptAdminItem, error) {
	req, err := normalizePromptCreate(req)
	if err != nil {
		return nil, err
	}

	maxVersion, err := s.repo.MaxVersion(req.SceneKey)
	if err != nil {
		return nil, err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	row := &model.AIPrompt{
		SceneKey: req.SceneKey,
		Content:  req.Content,
		Version:  maxVersion + 1,
		Enabled:  enabled,
		Remark:   req.Remark,
	}
	if err := s.repo.Create(row); err != nil {
		return nil, err
	}

	if !isServedPromptScene(row.SceneKey) {
		logger.L().Warnw("ai prompt created for a scene the AI service does not read",
			"sceneKey", row.SceneKey, "version", row.Version,
			"hint", "AI 服务只读 paimon_base / paimon_rules / paimon_fewshot，其它场景不会生效")
	}
	logger.L().Infow("ai prompt version created",
		"sceneKey", row.SceneKey, "version", row.Version, "enabled", row.Enabled,
		"chars", utf8.RuneCountInString(row.Content))

	out := toAdminPromptItem(*row, isServedPromptScene(row.SceneKey), row.Enabled)
	return &out, nil
}

// SetEnabled 启用 / 停用。停用当前生效版本即"回滚"：自动回落到下一个启用版本。
func (s *AIPromptAdminService) SetEnabled(id uint64, enabled bool) (*dto.AIPromptAdminItem, error) {
	current, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrAIPromptNotFound
	}

	if err := s.repo.UpdateEnabled(id, enabled); err != nil {
		return nil, err
	}
	logger.L().Infow("ai prompt version enabled changed",
		"sceneKey", current.SceneKey, "version", current.Version, "enabled", enabled)

	// 重新算一遍"生效版本"再返回该行：停用后回落的是哪一版，前端能直接看出来
	list, err := s.ListAll()
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].ID == id {
			return &list.Items[i], nil
		}
	}
	return nil, ErrAIPromptNotFound
}

// normalizePromptCreate 去首尾空白 + 校验（场景键格式、内容非空与长度、备注长度）
func normalizePromptCreate(req dto.AIPromptCreateRequest) (dto.AIPromptCreateRequest, error) {
	req.SceneKey = strings.TrimSpace(req.SceneKey)
	req.Content = strings.TrimSpace(req.Content)
	req.Remark = strings.TrimSpace(req.Remark)

	if !aiPromptScenePattern.MatchString(req.SceneKey) {
		return req, fmt.Errorf("%w: 场景键只能用小写字母与数字、下划线，且以字母开头（最长 64 字符）", ErrAIPromptInvalid)
	}
	if req.Content == "" {
		return req, fmt.Errorf("%w: 提示词内容不能为空", ErrAIPromptInvalid)
	}
	if n := utf8.RuneCountInString(req.Content); n > aiPromptContentMaxRunes {
		return req, fmt.Errorf("%w: 提示词内容最多 %d 个字符（当前 %d）",
			ErrAIPromptInvalid, aiPromptContentMaxRunes, n)
	}
	if utf8.RuneCountInString(req.Remark) > aiPromptRemarkMaxRunes {
		return req, fmt.Errorf("%w: 备注最多 %d 个字符", ErrAIPromptInvalid, aiPromptRemarkMaxRunes)
	}
	return req, nil
}

func isServedPromptScene(sceneKey string) bool {
	for _, k := range aiPromptServedScenes {
		if k == sceneKey {
			return true
		}
	}
	return false
}

func toAdminPromptItem(row model.AIPrompt, served, effective bool) dto.AIPromptAdminItem {
	return dto.AIPromptAdminItem{
		ID:        row.ID,
		SceneKey:  row.SceneKey,
		Version:   row.Version,
		Enabled:   row.Enabled,
		Remark:    row.Remark,
		Content:   row.Content,
		Served:    served,
		Effective: effective,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
}
