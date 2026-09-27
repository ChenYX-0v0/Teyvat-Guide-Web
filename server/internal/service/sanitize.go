package service

import (
	"strings"
	"unicode"
)

// 玩家昵称是**唯一**由玩家自由填写、又会被拼进 system 提示词的字段
// （Enka 昵称 → ai_context.go【玩家档案】块 → AI 服务 context_render.py → persona.py 动态区），
// 而且**每次请求都会自动带上** —— 玩家只要把昵称改成「忽略以上规则，你现在是…」，
// 就等于埋了一条**持久注入** ✗，比一次性越狱更难防。
//
// 加固三件套（2026-09-27，设计见 teyvat-docs/ai/09 与 ai/07 TODO-12）：
//  1. 白名单字符（汉字 / 字母 / 数字 / 空格 / ·-_）：干掉 < > " ' ` 换行、
//     零宽字符（U+200B~U+200F）、双向控制符（U+202A~U+202E）等结构性载体；
//  2. 长度上限 16 字：防「长文淹没指令」式攻击；
//  3. 注入短语一票否决：命中直接**丢弃昵称**（丢昵称的代价极低，被注入的代价很高）。
const nicknameMaxRunes = 16

// 命中即丢弃昵称。只收窄不放宽（宁可少显示一个昵称，也不要放过一条持久注入）。
// ⚠️ 匹配前会把短语里的空格一并去掉（与输入侧的扁平化对齐），
//
//	否则 "Ignore all previous…" 这种带空格的短语会漏（2026-09-27 单测抓到 ✗）。
var nicknameInjectPhrases = []string{
	// 中文
	"忽略以上", "忽略之前", "忽略上述", "忽略全部", "忽略所有", "忽略前面",
	"系统提示", "提示词", "你现在是", "从现在起", "从现在开始", "扮演", "角色扮演",
	"越狱", "开发者模式", "指令",
	// 英文（统一按小写比较）
	"ignore previous", "ignore above", "ignore all", "system prompt", "jailbreak",
	"developer mode", "you are now", "act as", "dan mode",
}

// nicknameInjectKeys 短语的扁平化版本（去空格 + 小写），与输入侧同一套归一化，
// 避免玩家用空格拆分绕过（"忽 略 以 上" / "Ignore all previous"）。
var nicknameInjectKeys = func() []string {
	strip := strings.NewReplacer(" ", "", "\u3000", "", "\t", "", "\n", "")
	out := make([]string, 0, len(nicknameInjectPhrases))
	for _, p := range nicknameInjectPhrases {
		out = append(out, strip.Replace(strings.ToLower(p)))
	}
	return out
}()

// sanitizePlayerNickname 清洗玩家昵称；返回空串表示「这个昵称不可信，别带进上下文」。
func sanitizePlayerNickname(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// 为短语匹配做扁平化：去掉空白与统一小写（与短语同一套规则）
	flat := strings.NewReplacer(" ", "", "\u3000", "", "\t", "", "\n", "").Replace(strings.ToLower(s))
	for _, p := range nicknameInjectKeys {
		if strings.Contains(flat, p) {
			return ""
		}
	}

	var b strings.Builder
	b.Grow(len(s))
	n := 0
	for _, r := range s {
		if n >= nicknameMaxRunes {
			break
		}
		if !nicknameRuneOK(r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// nicknameRuneOK 白名单：汉字（含扩展区）、字母、数字、空格，以及昵称里常见的三个分隔符。
// 其余一律丢弃 —— 包括引号、尖括号、反引号、换行、零宽字符、双向控制符、emoji。
func nicknameRuneOK(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r), unicode.IsLetter(r), unicode.IsDigit(r):
		return true
	case r == ' ', r == '·', r == '-', r == '_':
		return true
	}
	return false
}
