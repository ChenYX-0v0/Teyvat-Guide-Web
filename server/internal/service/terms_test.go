package service

import (
	"errors"
	"testing"
)

// 版本号校验必须在**触库之前**完成：旧页面 / 伪造请求一律拒绝，且不需要数据库即可测。
// （注意：不带 repo 的 AuthService 只能测"被拒绝"的路径；"通过"的路径会走到 DB，
//  所以这里只覆盖拒绝分支 —— 那正是安全边界所在。）
func TestAcceptTermsRejectsWrongVersion(t *testing.T) {
	svc := &AuthService{}
	bad := []string{
		"",
		"   ",
		"v0.9-20200101",            // 旧版本
		"V1.0-20260927",            // 大小写不同（不做宽容匹配）
		CurrentTermsVersion + "-x", // 前缀相同但不是本版本
		"x" + CurrentTermsVersion,  // 多了前缀
	}
	for _, v := range bad {
		if err := svc.AcceptTerms(1, v); !errors.Is(err, ErrTermsVersionInvalid) {
			t.Fatalf("版本 %q 应被拒绝（ErrTermsVersionInvalid），实际 = %v", v, err)
		}
	}
}

// uid = 0（未登录）不写库，直接拒绝
func TestAcceptTermsRejectsUnknownUser(t *testing.T) {
	svc := &AuthService{}
	if err := svc.AcceptTerms(0, CurrentTermsVersion); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("uid=0 应返回 ErrUserNotFound，实际 = %v", err)
	}
}

// 版本号本身必须是非空、可读的固定串（防止误改成空字符串导致"人人已同意"）
func TestCurrentTermsVersionShape(t *testing.T) {
	if len(CurrentTermsVersion) < 8 {
		t.Fatalf("CurrentTermsVersion 过短：%q", CurrentTermsVersion)
	}
}
