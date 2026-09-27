package service

// 邀请码（注册的第二道门，2026-09-28）
//
// 规则真源（用户 2026-09-28 定 ✓）：
//  1. 注册表单里**邀请码在手机号上方** ✓；
//  2. **不填不能发短信** ✓、**码不对也不能发短信** ✓ ⇒ 发码那一步就校验 ✓（错的码连短信都不会发 ✗）；
//  3. **一次性**：用它注册成功即作废 ✓ —— 占用发生在注册的事务里（见 repository.CreateUserWithInvite ✓）。
//
// 为什么发码阶段只校验、不占用 ✗：发完码用户可能填错短信码或直接放弃 ✓，
// 那时把码烧掉对用户不公平（码是"准入凭证"，不是"发短信凭证"✓）。

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/repository"
)

// inviteAlphabet 去掉易混字符（0/O、1/I/L 之类 ✗）—— 邀请码常要人工转达，抄错很常见 ✓
const inviteAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// InviteCodeLen 邀请码长度（8 位 ≈ 32^8 ≈ 1.1e12 组合 ✓；配三重限速足够防爆破 ✓）
const InviteCodeLen = 8

var (
	// ErrInviteRequired 需要邀请码但没填（注册开关 + require_invite_code 都开着时 ✓）
	ErrInviteRequired = errors.New("请填写邀请码")
	// ErrInviteInvalid 码不存在 / 已停用（**不区分**，不给试探者额外信息 ✓）
	ErrInviteInvalid = errors.New("邀请码不存在或已停用，请核对后再试")
	// ErrInviteUsed 码已被别人用过（一次性 ✓）
	ErrInviteUsed = errors.New("该邀请码已被使用，请换一个")
)

// NormalizeInviteCode 归一化邀请码：去空白与连字符、转大写（用户手抄时大小写常混 ✓）
func NormalizeInviteCode(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.NewReplacer(" ", "", "-", "", "—", "", "_", "").Replace(s)
	return strings.ToUpper(s)
}

// validInviteFormat 格式校验：长度对 + 只含字母表内字符（先挡掉明显乱填的 ✓）
func validInviteFormat(code string) bool {
	if len(code) != InviteCodeLen {
		return false
	}
	for _, r := range code {
		if !strings.ContainsRune(inviteAlphabet, r) {
			return false
		}
	}
	return true
}

// InviteService 邀请码：校验（发码/注册用）+ 生成与管理（后台用）
type InviteService struct {
	repo *repository.InviteRepository
}

func NewInviteService() *InviteService {
	return &InviteService{repo: repository.NewInviteRepository()}
}

// Check 校验邀请码**当前可用**（启用 + 未使用 ✓），**不占用** ✓。
// 空码 → ErrInviteRequired（前端"不填不能发短信"的服务端对应 ✓）；
// 格式不对 → 直接 ErrInviteInvalid（省一次查询 ✓）；查不到/停用 → ErrInviteInvalid ✓；已用 → ErrInviteUsed ✓。
func (s *InviteService) Check(rawCode string) error {
	code := NormalizeInviteCode(rawCode)
	if code == "" {
		return ErrInviteRequired
	}
	if !validInviteFormat(code) {
		return ErrInviteInvalid
	}
	c, err := s.repo.Get(code)
	if err != nil {
		return err
	}
	if c == nil || !c.Enabled {
		return ErrInviteInvalid
	}
	if c.Used() {
		return ErrInviteUsed
	}
	return nil
}

// Generate 生成一批邀请码（后台用）。撞码就重试（概率极低，但别假装不可能 ✓）。
func (s *InviteService) Generate(note string, count int) ([]model.InviteCode, error) {
	if count <= 0 || count > 100 {
		count = 1
	}
	out := make([]model.InviteCode, 0, count)
	for i := 0; i < count; i++ {
		created := false
		for attempt := 0; attempt < 5 && !created; attempt++ {
			code, err := randomInviteCode()
			if err != nil {
				return nil, err
			}
			item := model.InviteCode{Code: code, Note: strings.TrimSpace(note), Enabled: true}
			if err := s.repo.Create(&item); err != nil {
				continue // 撞主键（或 DB 抖动）：换一个码再来 ✓
			}
			out = append(out, item)
			created = true
		}
		if !created {
			return out, fmt.Errorf("生成邀请码失败（连续 5 次撞码或数据库不可用）")
		}
	}
	return out, nil
}

// List 列出（unused=true 只看可用 ✓）
func (s *InviteService) List(unused bool, limit int) ([]model.InviteCode, error) {
	return s.repo.List(unused, limit)
}

// CountUsable 可用码数量
func (s *InviteService) CountUsable() (int64, error) { return s.repo.CountUsable() }

// SetEnabled 停用 / 启用；码不存在返回 ErrInviteInvalid ✓
func (s *InviteService) SetEnabled(rawCode string, enabled bool) error {
	code := NormalizeInviteCode(rawCode)
	if code == "" {
		return ErrInviteInvalid
	}
	n, err := s.repo.SetEnabled(code, enabled)
	if err != nil {
		return err
	}
	if n == 0 {
		// 要么不存在，要么已经是目标状态 —— 用 Get 区分，避免"停用成功了却报错"✗
		c, err := s.repo.Get(code)
		if err != nil {
			return err
		}
		if c == nil {
			return ErrInviteInvalid
		}
	}
	return nil
}

// randomInviteCode 生成邀请码（crypto/rand，别用 math/rand ✗）
func randomInviteCode() (string, error) {
	var b strings.Builder
	for i := 0; i < InviteCodeLen; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteAlphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(inviteAlphabet[n.Int64()])
	}
	return b.String(), nil
}
