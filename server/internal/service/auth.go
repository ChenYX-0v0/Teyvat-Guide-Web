package service

// 认证服务：注册 / 登录（JWT）/ 种子账号初始化

import (
	"errors"
	"fmt"
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
)

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

// Register 注册新用户
func (s *AuthService) Register(username, password, displayName string) (*model.User, error) {
	if len(username) < 3 || len(username) > 32 {
		return nil, errors.New("用户名长度需为 3~32 个字符")
	}
	if len(password) < 6 {
		return nil, errors.New("密码长度至少 6 位")
	}
	if existing, err := s.repo.FindByUsername(username); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrUsernameTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  firstNonEmpty(displayName, username),
	}
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
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
