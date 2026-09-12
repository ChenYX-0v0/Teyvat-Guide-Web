package service

import (
	"strings"
	"testing"
	"time"

	"teyvat-guide/server/internal/model"
)

func newMsg(role, content string) model.ChatMessage {
	return model.ChatMessage{Role: role, Content: content, CreatedAt: time.Now()}
}

// 本次提问已落库又通过 prompt 单独下发，组装历史时必须丢弃尾部那条，避免重复计费
func TestBuildHistoryDropsCurrentQuestion(t *testing.T) {
	recent := []model.ChatMessage{
		newMsg("user", "胡桃怎么玩"),
		newMsg("assistant", "胡桃是火系主C"),
		newMsg("user", "那圣遗物怎么选"),
	}

	got := buildHistory(recent, "那圣遗物怎么选")

	if len(got) != 2 {
		t.Fatalf("期望 2 条历史，实际 %d 条：%+v", len(got), got)
	}
	if got[0].Content != "胡桃怎么玩" || got[1].Content != "胡桃是火系主C" {
		t.Fatalf("历史顺序/内容不符：%+v", got)
	}
}

// 降级兜底文案不承载有效信息，留在上下文里会污染后续对话
func TestBuildHistoryFiltersDegradeReply(t *testing.T) {
	recent := []model.ChatMessage{
		newMsg("user", "在吗"),
		newMsg("assistant", aiDegradeReply),
		newMsg("user", "胡桃怎么玩"),
	}

	got := buildHistory(recent, "胡桃怎么玩")

	if len(got) != 1 || got[0].Content != "在吗" {
		t.Fatalf("期望只保留 [在吗]，实际：%+v", got)
	}
}

// 从最近一条往前累计，超预算即截断（保留更近的上下文）
func TestBuildHistoryTrimsByTokenBudget(t *testing.T) {
	recent := []model.ChatMessage{
		newMsg("user", strings.Repeat("旧", 500)),
		newMsg("assistant", strings.Repeat("答", 800)),
		newMsg("user", "最新问题"),
	}

	got := buildHistory(recent, "")

	if len(got) != 2 {
		t.Fatalf("期望 2 条（超预算的旧问题被截断），实际 %d 条：%+v", len(got), got)
	}
	if got[0].Role != "assistant" || got[1].Content != "最新问题" {
		t.Fatalf("历史顺序/内容不符：%+v", got)
	}
}

// 最新一条即使超预算也必须保留，否则模型会丢失最近的语境
func TestBuildHistoryKeepsNewestOverBudget(t *testing.T) {
	recent := []model.ChatMessage{
		newMsg("user", strings.Repeat("长", historyTokenBudget*2)),
	}

	got := buildHistory(recent, "")

	if len(got) != 1 {
		t.Fatalf("期望保留最新一条，实际：%+v", got)
	}
}

func TestBuildHistoryEmpty(t *testing.T) {
	if got := buildHistory(nil, "随便问问"); len(got) != 0 {
		t.Fatalf("空历史应返回空切片，实际：%+v", got)
	}
}
