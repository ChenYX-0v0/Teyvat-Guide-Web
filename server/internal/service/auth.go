package service

// 认证服务：注册 / 登录（JWT）/ 种子账号初始化

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUsernameTaken  = errors.New("用户名已存在")
	ErrBadCredentials = errors.New("用户名或密码错误")
	ErrUserBanned     = errors.New("账号已被封禁")
	// ErrUserInvalid 账号参数不合法（用户名长度 / 密码长度等），Handler 映射 400
	ErrUserInvalid = errors.New("账号参数不合法")
	// ErrRegisterClosed 公开注册已关闭（不公开 / 邀请制部署）：账号改由后台「新增用户」创建
	ErrRegisterClosed = errors.New("本站为不公开邀请制，已关闭注册；请联系管理员开通账号")
	// ErrPhoneTaken 手机号已注册（手机号注册查重，Handler 映射 409）
	ErrPhoneTaken = errors.New("该手机号已注册，请直接登录")
	// ErrOldPasswordWrong 改密时"当前密码"填错（Handler 映射 **400** 而不是 401 ✓）：
	// 401 会被前端当成"登录态失效"⇒ 直接把用户踢到登录页 ✗，而这里只是表单填错了 ✓。
	ErrOldPasswordWrong = errors.New("当前密码不正确")
	// ErrPasswordSame 新密码与当前密码相同（改了个寂寞 ⇒ 直说 ✓）
	ErrPasswordSame = errors.New("新密码不能与当前密码相同")
	// 注：ErrUserNotFound（用户不存在）定义在 admin_users.go，同一包内复用
)

// displayNameMaxRunes 昵称上限（**按字符计**、不是字节 ✓，与 preset.go 的口径一致）。
// model.User.DisplayName 是 size:64 ⇒ 这里留足余量 ✓。
const displayNameMaxRunes = 24

// validatePassword 密码强度校验（**唯一真源** ✓）：注册 / 后台代建 / 修改密码都走这里，
// 避免三处各写一遍后规则漂移 ✗（本仓反复强调的口径）。
func validatePassword(password string) error {
	if len(password) < 6 {
		return fmt.Errorf("%w：密码长度至少 6 位", ErrUserInvalid)
	}
	return nil
}

// buildUser 账号参数校验 + bcrypt 哈希 + 组装（**不落库**）。
// 校验规则的**唯一真源**：用户名注册 / 手机号注册 / 后台代建三条路径都走这里，
// 避免三份校验与哈希各写一遍后互相漂移 ✗。
func buildUser(username, password, displayName string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 32 {
		return nil, fmt.Errorf("%w：用户名长度需为 3~32 个字符", ErrUserInvalid)
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &model.User{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  firstNonEmpty(strings.TrimSpace(displayName), username),
	}, nil
}

// createUserWithPassword 校验 + 查重 + 落库（后台代建 / 用户名注册共用）
func createUserWithPassword(repo *repository.UserRepository, username, password, displayName string) (*model.User, error) {
	u, err := buildUser(username, password, displayName)
	if err != nil {
		return nil, err
	}
	existing, err := repo.FindByUsername(u.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameTaken
	}
	if err := repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

// RegisterOptions 注册入口的可用性快照（`GET /api/auth/config` 下发）。
// 前端据此决定显示"注册"入口还是"邀请制"提示 ✓；`RequireInviteCode` 为**邀请码功能**预留的开关位
// （当前恒 false —— 先做常规注册，邀请码后续再开 ✓）。
type RegisterOptions struct {
	AllowRegister         bool `json:"allowRegister"`
	SMSEnabled            bool `json:"smsEnabled"`
	SMSMock               bool `json:"smsMock"`
	SMSCodeTTLSeconds     int  `json:"smsCodeTTLSeconds"`
	SMSResendAfterSeconds int  `json:"smsResendAfterSeconds"`
	RequireInviteCode     bool `json:"requireInviteCode"`
}

type AuthService struct {
	cfg     config.AuthConfig
	repo    *repository.UserRepository
	sms     *SMSService
	invites *InviteService
	ttl     time.Duration
	secret  []byte
}

// NewAuthService 构造。sms 可以为 nil（短信未配置时注册接口会明确拒绝 ✓，其余功能不受影响 ✓）。
func NewAuthService(cfg config.AuthConfig, sms *SMSService, db *gorm.DB) *AuthService {
	s := &AuthService{
		cfg:     cfg,
		repo:    repository.NewUserRepository(),
		sms:     sms,
		invites: NewInviteService(),
		ttl:     time.Duration(cfg.JWTTTLHours) * time.Hour,
		secret:  []byte(cfg.JWTSecret),
	}
	s.seedIfEmpty()
	return s
}

// RequireInviteCode 是否要求邀请码（配置开关 ✓）。
// 前端不硬编码：`GET /api/auth/config` 的 `requireInviteCode` 就是它 ✓。
func (s *AuthService) RequireInviteCode() bool { return s.cfg.RequireInviteCode }

// seedIfEmpty 用户表为空时创建种子账号（默认 admin/admin123，请登录后尽快修改）
func (s *AuthService) seedIfEmpty() {
	n, err := s.repo.Count()
	if err != nil {
		logger.L().Warnw("user count failed, skip seed", "err", err)
		return
	}
	if n > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.cfg.SeedPass), bcrypt.DefaultCost)
	if err != nil {
		logger.L().Warnw("seed password hash failed", "err", err)
		return
	}
	if err := s.repo.Create(&model.User{
		Username:     s.cfg.SeedUser,
		PasswordHash: string(hash),
		DisplayName:  "管理员",
	}); err != nil {
		logger.L().Warnw("seed user create failed", "err", err)
		return
	}
	logger.L().Infow("seed user created", "username", s.cfg.SeedUser)
}

// Register 手机号注册（2026-09-27 改为"手机号 + 短信验证码 + 设置密码"）。
// ⚠️ **不公开 / 邀请制部署下这里是关闭的**（`auth.allow_register=false`，本项目当前口径）：
// 账号只能由后台「用户管理 → 新增用户」代建（`UserAdminService.CreateUser`），
// 该路径不受此开关限制——否则邀请制部署就没法开户了。口径见 shared-docs/04 §2.3。
//
// 为什么用手机号当用户名：登录链路零改动 ✓（`Login` 仍按 username 查），
// 且用户不用再记一个自造的用户名 ✓。
//
// ⚠️ 顺序刻意是"**先验码、再查重**"：反过来的话，"这个手机号注册过没有"就成了
// **不需要持码就能探测**的信息 ✗（可被拿来批量判断哪些号码是本站用户）。
func (s *AuthService) Register(phone, code, password, displayName, inviteCode string) (*model.User, error) {
	if !s.cfg.AllowRegister {
		return nil, ErrRegisterClosed
	}
	if s.sms == nil || !s.sms.Enabled() {
		return nil, ErrSMSDisabled
	}
	normalized, ok := NormalizePhone(phone)
	if !ok {
		return nil, ErrPhoneInvalid
	}
	// ① 邀请码（准入）—— 只校验，**先不占用** ✓：万一后面短信码填错，码不该被烧掉 ✗
	if s.RequireInviteCode() {
		if err := s.invites.Check(inviteCode); err != nil {
			return nil, err
		}
	}
	// ② 短信验证码（持号证明 ✓）
	if err := s.sms.VerifyCode(normalized, code); err != nil {
		return nil, err
	}
	// ③ 查重（便宜的先做：省得白占用一个码 ✓）
	if existing, err := s.repo.FindByPhone(normalized); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrPhoneTaken
	}
	// 昵称缺省 = 脱敏手机号（138****5678 ✓），避免把完整号码当昵称直接展示
	u, err := buildUser(normalized, password, firstNonEmpty(strings.TrimSpace(displayName), MaskPhone(normalized)))
	if err != nil {
		return nil, err
	}
	u.Phone = &normalized

	// ④ 建号 +（需要时）**原子占用邀请码** —— 同一个事务，任何一步失败整体回滚 ✓✓
	//    于是：并发下不会两个账号共用一个码 ✓；建号失败也不会把码白白烧掉 ✓。
	if s.RequireInviteCode() {
		err = s.invites.repo.CreateUserWithInvite(u, NormalizeInviteCode(inviteCode), normalized)
		if errors.Is(err, repository.ErrInviteTaken) {
			return nil, ErrInviteUsed
		}
		if err != nil {
			// 手机号并发撞唯一索引 ⇒ 仍然回"已注册"（不是 500 ✗）
			if again, e2 := s.repo.FindByPhone(normalized); e2 == nil && again != nil {
				return nil, ErrPhoneTaken
			}
			return nil, err
		}
		return u, nil
	}
	if err := s.repo.Create(u); err != nil {
		// 并发下靠 phone 唯一索引兜底：撞了就回"已注册"（否则会 500 ✗）
		if again, e2 := s.repo.FindByPhone(normalized); e2 == nil && again != nil {
			return nil, ErrPhoneTaken
		}
		return nil, err
	}
	return u, nil
}

// SendSMSCode 发送注册验证码（薄封装：Handler 只依赖 AuthService ✓，不直接碰短信通道）
//
// ⚠️ 注册关闭时**不发电**：验证码唯一用途就是注册 ✗ —— 否则"不公开部署"仍会被刷短信费 ✗
// （2026-09-27 抽查发现：只关 allow_register 时本接口照样 200 ⇒ 成本漏洞 ✓ 已堵）。
func (s *AuthService) SendSMSCode(phone, clientIP, inviteCode string) error {
	if !s.cfg.AllowRegister {
		return ErrRegisterClosed
	}
	// 邀请码先于一切：**不填 / 不对 ⇒ 连短信都不发** ✓（2026-09-28 用户口径 ✓）。
	// 顺带也避免把本接口当成免费的"邀请码试探器"✗（错的码不会触发任何发送与计费 ✓）。
	if s.RequireInviteCode() {
		if err := s.invites.Check(inviteCode); err != nil {
			return err
		}
	}
	if s.sms == nil {
		return ErrSMSDisabled
	}
	return s.sms.SendCode(phone, clientIP)
}

// RegisterOptions 注册入口可用性（`GET /api/auth/config`；前端决定显示注册入口还是邀请制提示 ✓）
func (s *AuthService) RegisterOptions() RegisterOptions {
	opts := RegisterOptions{
		AllowRegister:     s.cfg.AllowRegister,
		RequireInviteCode: s.cfg.RequireInviteCode,
	}
	if s.sms != nil {
		opts.SMSEnabled = s.sms.Enabled()
		opts.SMSMock = s.sms.Mock()
		opts.SMSCodeTTLSeconds = s.sms.TTLSeconds()
		opts.SMSResendAfterSeconds = s.sms.SendIntervalSeconds()
	}
	return opts
}

// Login 校验账密并签发 JWT
func (s *AuthService) Login(username, password string) (token, displayName string, err error) {
	u, err := s.repo.FindByUsername(username)
	if err != nil {
		return "", "", err
	}
	if u == nil {
		return "", "", ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrBadCredentials
	}
	if u.IsBanned() {
		return "", "", ErrUserBanned
	}

	if err := s.repo.TouchLogin(u.ID); err != nil {
		logger.L().Warnw("update last login failed", "user", u.ID, "err", err)
	}

	claims := jwt.MapClaims{
		"uid":      u.ID,
		"username": u.Username,
		"exp":      time.Now().Add(s.ttl).Unix(),
		"iat":      time.Now().Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", "", err
	}
	return signed, u.DisplayName, nil
}

// Profile 按 ID 取用户（`/api/auth/me` 用：返回昵称、绑定 UID 与用户须知状态）。
// 与 Login 的区别：这里不校验密码，只做"Token 有效 → 用户仍在"的确认。
func (s *AuthService) Profile(userID uint) (*model.User, error) {
	if userID == 0 {
		return nil, ErrUserNotFound
	}
	u, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// ParseUserID 从 JWT 中解析用户 ID（JWTAuth 中间件已校验签名）
func (s *AuthService) ParseUserID(tokenStr string) (uint, string, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !t.Valid {
		return 0, "", ErrBadCredentials
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", ErrBadCredentials
	}
	uidF, _ := claims["uid"].(float64)
	username, _ := claims["username"].(string)
	return uint(uidF), username, nil
}

// UpdateDisplayName 修改昵称（`PUT /api/auth/profile`，2026-10-11）。
//
// 口径：前后空格一律 TrimSpace ✓（否则"小明"与"小明 "会变成两个看起来一样的昵称 ✗）；
// 空串 = 清空昵称（前端回落到用户名 / 脱敏手机号展示 ✓）；上限按**字符**计 ✓；
// 值没变就直接返回，不写库（幂等 ✓）。
func (s *AuthService) UpdateDisplayName(userID uint, displayName string) (*model.User, error) {
	if userID == 0 {
		return nil, ErrUserNotFound
	}
	name := strings.TrimSpace(displayName)
	if len([]rune(name)) > displayNameMaxRunes {
		return nil, fmt.Errorf("%w：昵称最多 %d 个字符", ErrUserInvalid, displayNameMaxRunes)
	}
	u, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	if u.DisplayName == name {
		return u, nil
	}
	if err := s.repo.UpdateDisplayName(userID, name); err != nil {
		return nil, err
	}
	u.DisplayName = name
	return u, nil
}

// ChangePassword 修改密码（`POST /api/auth/password`，2026-10-11）：
// 先验**旧密码**，再按**同一套**强度规则（validatePassword ✓，与注册共用 ⇒ 不会规则漂移 ✗）校验新密码，
// 最后写入新哈希。
//
// ⚠️ 不会吊销既有 Token ✗ —— JWT 里只有 uid/username/exp，没有密码版本位；
// "改密即踢下线"要加列 + 改中间件，属独立一块（Handler 注释与 docs/web/04 §4.2 都写明了 ✓）。
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	if userID == 0 {
		return ErrUserNotFound
	}
	u, err := s.repo.FindByID(userID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrOldPasswordWrong
	}
	if oldPassword == newPassword {
		return ErrPasswordSame
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordHash(userID, string(hash))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
