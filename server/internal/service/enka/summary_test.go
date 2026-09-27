package enka

import "testing"

// 指针语义是这个特性的关键：玩家隐藏战绩时 Enka **不下发**字段，
// 必须与"合法值 0"区分开 —— 0 层 / 0 星 是合法值，不能当成"没有"。
func TestEndgameSummariesDistinguishMissingFromZero(t *testing.T) {
	// ① 字段完全缺失（玩家隐藏战绩）→ 三个概要都应为 nil
	var empty PlayerInfo
	if abyssSummary(empty) != nil || theaterSummary(empty) != nil || stygianSummary(empty) != nil {
		t.Fatal("字段缺失时应返回 nil（区分于 0），实际拿到了非 nil")
	}

	// ② 字段存在但值就是 0（例如新号还没打过）→ 应返回非 nil、值为 0
	zero := 0
	pi := PlayerInfo{
		TowerFloorIndex:  &zero,
		TheaterActIndex:  &zero,
		StygianIndex:     &zero,
	}
	if abyssSummary(pi) == nil || theaterSummary(pi) == nil || stygianSummary(pi) == nil {
		t.Fatal("字段存在且值为 0 时应返回非 nil")
	}

	// ③ 实测样本（UID 197794747，2026-09-27）：12-3 满星 36 / 剧诗第 10 幕 10 星 / 危战 6、324 秒
	floor, level, stars := 12, 3, 36
	act, mode, tStars := 10, 119, 10
	idx, secs, id := 6, 324, 5269011
	full := PlayerInfo{
		TowerFloorIndex: &floor, TowerLevelIndex: &level, TowerStarIndex: &stars,
		TheaterActIndex: &act, TheaterModeIndex: &mode, TheaterStarIndex: &tStars,
		StygianIndex: &idx, StygianSeconds: &secs, StygianID: &id,
	}
	a := abyssSummary(full)
	if a == nil || a.Floor != 12 || a.Room != 3 || a.Stars != 36 {
		t.Fatalf("深渊概要翻译不符：%+v", a)
	}
	th := theaterSummary(full)
	if th == nil || th.Act != 10 || th.Stars != 10 {
		t.Fatalf("剧诗概要翻译不符：%+v", th)
	}
	st := stygianSummary(full)
	if st == nil || st.Index != 6 || st.Seconds != 324 || st.ID != 5269011 {
		t.Fatalf("危战概要翻译不符：%+v", st)
	}
}
