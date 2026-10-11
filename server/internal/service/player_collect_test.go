package service

// 「已收录角色」（2026-10-11，用户口径）：
//   展柜最多 12 位 ⇒ 用户会**轮换展柜**；所以角色池必须**只增 / 只改，永不删** ✓，
//   这样用户反复换展柜就能把角色逐步**全部导入**进来 ✓。
//
// 这里验的就是这条口径的四个面：不丢 / 会刷新 / 幂等 / 换回来能复活标记 ✓。

import (
	"testing"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/repository"
)

func TestCollectCharactersAccumulatesNeverDeletes(t *testing.T) {
	setupUsageTestDB(t)
	// setupUsageTestDB 只迁了 User/会话/预设/邀请码 ⇒ 这张表要在用例里自己迁 ✓
	if err := database.DB().AutoMigrate(&model.PlayerCharacter{}); err != nil {
		t.Fatalf("迁移 player_characters 失败：%v", err)
	}
	svc := &PlayerService{repo: repository.NewPlayerRepository()}
	uid := "197794747"

	card := func(id int64, name string, level int) dto.CharacterCard {
		return dto.CharacterCard{ID: id, Name: name, Level: level}
	}

	// ① 第一轮展柜：A / B
	svc.CollectCharacters(uid, []dto.CharacterCard{card(1, "A", 80), card(2, "B", 70)})
	col, err := svc.Characters(uid)
	if err != nil {
		t.Fatalf("读取已收录角色失败：%v", err)
	}
	if col.Total != 2 || col.ShowcaseCount != 2 {
		t.Fatalf("第一轮应为 总数2/展柜2，实际 %d/%d", col.Total, col.ShowcaseCount)
	}

	// ② 换展柜：B（顺手练到 90）+ C ⇒ **A 必须还在**（只是被标成"不在展柜" ✓）
	svc.CollectCharacters(uid, []dto.CharacterCard{card(2, "B", 90), card(3, "C", 60)})
	col, _ = svc.Characters(uid)
	if col.Total != 3 {
		t.Fatalf("换展柜后总数应累积为 3（A 不能丢 ✗），实际 %d", col.Total)
	}
	if col.ShowcaseCount != 2 {
		t.Fatalf("本轮展柜应为 2，实际 %d", col.ShowcaseCount)
	}
	// 排序：展柜中的在前 ⇒ 前两个在展柜、第三个（A）不在 ✓
	if !col.Characters[0].InShowcase || !col.Characters[1].InShowcase || col.Characters[2].InShowcase {
		t.Fatalf("排序应为'展柜中的在前'，实际：%+v", col.Characters)
	}
	// 仍在展柜里的 B 应被**刷新**到 90 ✓（"只改" ✓）
	var b dto.CollectedCharacter
	for _, c := range col.Characters {
		if c.ID == 2 {
			b = c
		}
	}
	if b.Level != 90 {
		t.Fatalf("同一角色再见到时应刷新，B.Level 应为 90，实际 %d", b.Level)
	}
	if b.FirstSeenAt.IsZero() || b.LastSeenAt.IsZero() {
		t.Fatalf("应带首次/最近收录时间 ✓，实际 %+v", b)
	}

	// ③ 同一展柜再采一次：幂等（总数与展柜数都不变 ✓）
	svc.CollectCharacters(uid, []dto.CharacterCard{card(2, "B", 90), card(3, "C", 60)})
	col, _ = svc.Characters(uid)
	if col.Total != 3 || col.ShowcaseCount != 2 {
		t.Fatalf("重复采集应幂等，实际 %d/%d", col.Total, col.ShowcaseCount)
	}

	// ④ 换回 A / B：A 重新变成"在展柜" ✓（它的行一直都在 ✓）
	svc.CollectCharacters(uid, []dto.CharacterCard{card(1, "A", 80), card(2, "B", 90)})
	col, _ = svc.Characters(uid)
	if col.Total != 3 || col.ShowcaseCount != 2 {
		t.Fatalf("换回后应仍是 总数3/展柜2，实际 %d/%d", col.Total, col.ShowcaseCount)
	}
	revived := false
	for _, c := range col.Characters {
		if c.ID == 1 && c.InShowcase {
			revived = true
		}
	}
	if !revived {
		t.Fatalf("A 换回展柜后应重新标记为在展柜 ✓")
	}

	// ⑤ 健壮性：空入参不炸 ✓；不同 UID 之间**按 UID 隔离** ✓
	svc.CollectCharacters(uid, nil)
	other, err := svc.Characters("100000000")
	if err != nil {
		t.Fatalf("读别的 UID 不该报错：%v", err)
	}
	if other.Total != 0 {
		t.Fatalf("另一个 UID 不该看到这些角色，实际 %d", other.Total)
	}
}
