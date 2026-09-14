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
)

// createUserWithPassword 校验 + bcrypt 哈希 + 落库。
// **公开注册（AuthService.Register）与后台代建（UserAdminService.CreateUser）共用同一套规则**，
// 避免两处校验/哈希逻辑各自漂移。
func createUserWithPassword(repo *repository.UserRepository, username, password, displayName string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 32 {
		return nil, fmt.Errorf("%w：用户名长度需为 3~32 个字符", ErrUserInvalid)
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("%w：密码长度至少 6 位", ErrUserInvalid)
	}
	existing, err := repo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  firstNonEmpty(strings.TrimSpace(displayName), username),
	}
	if err := repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

type AuthService struct {
	cfg    config.AuthConfig
	repo   *repository.UserRepository
	ttl    time.Duration
	secret []byte
}

func NewAuthService(cfg config.AuthConfig, db *gorm.DB) *AuthService {
	s := &AuthService{
		cfg:    cfg,
		repo:   repository.NewUserRepository(),
		ttl:    time.Duration(cfg.JWTTTLHours) * time.Hour,
		secret: []byte(cfg.JWTSecret),
	}
	s.seedIfEmpty()
	return s
}

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

// Register 公开注册。
// ⚠️ **不公开 / 邀请制部署下这里是关闭的**（`auth.allow_register=false`，本项目当前口径）：
// 账号只能由后台「用户管理 → 新增用户」代建（`UserAdminService.CreateUser`），
// 该路径不受此开关限制——否则邀请制部署就没法开户了。口径见 shared-docs/04 §2.3。
func (s *AuthService) Register(username, password, displayName string) (*model.User, error) {
	if !s.cfg.AllowRegister {
		return nil, ErrRegisterClosed
	}
	return createUserWithPassword(s.repo, username, password, displayName)
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
