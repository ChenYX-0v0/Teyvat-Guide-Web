package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"teyvat-guide/server/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func signToken(t *testing.T, secret string, uid uint, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":      float64(uid),
		"username": "tester",
		"exp":      exp.Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("签发测试 Token 失败：%v", err)
	}
	return signed
}

// 关键行为：未带 Token 才允许匿名；**带了无效/过期 Token 必须 401**，不得静默降级为匿名
func TestJWTAuthOptional(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"
	cfg := config.AuthConfig{JWTSecret: secret}

	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantUID    uint
	}{
		{name: "未带 Token → 放行且匿名", header: "", wantStatus: http.StatusOK, wantUID: 0},
		{name: "非 Bearer 头 → 视为未带 Token", header: "Basic abc", wantStatus: http.StatusOK, wantUID: 0},
		{name: "非法 Token → 401（不静默匿名）", header: "Bearer not-a-jwt", wantStatus: http.StatusUnauthorized, wantUID: 0},
		{
			name:       "过期 Token → 401",
			header:     "Bearer " + signToken(t, secret, 7, time.Now().Add(-time.Hour)),
			wantStatus: http.StatusUnauthorized,
			wantUID:    0,
		},
		{
			name:       "签名不匹配 → 401",
			header:     "Bearer " + signToken(t, "another-secret", 7, time.Now().Add(time.Hour)),
			wantStatus: http.StatusUnauthorized,
			wantUID:    0,
		},
		{
			name:       "有效 Token → 注入 uid",
			header:     "Bearer " + signToken(t, secret, 7, time.Now().Add(time.Hour)),
			wantStatus: http.StatusOK,
			wantUID:    7,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			var gotUID uint
			r.GET("/probe", JWTAuthOptional(cfg, nil), func(c *gin.Context) {
				gotUID = c.GetUint("uid")
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/probe", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("期望 HTTP %d，实际 %d（body=%s）", tc.wantStatus, w.Code, w.Body.String())
			}
			if gotUID != tc.wantUID {
				t.Fatalf("期望注入 uid=%d，实际 %d", tc.wantUID, gotUID)
			}
		})
	}
}
