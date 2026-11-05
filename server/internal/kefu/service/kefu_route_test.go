package service

import (
	"testing"
)

// TestUniqueIDs 校验关联链提交的分类与渠道 id 去重去 0 且保持顺序
func TestUniqueIDs(t *testing.T) {
	if got := uniqueIDs(nil); len(got) != 0 {
		t.Fatalf("expect empty ids, got %v", got)
	}

	got := uniqueIDs([]int64{3, 0, 3, 5, -1, 5})
	if len(got) != 2 || got[0] != 3 || got[1] != 5 {
		t.Fatalf("unexpected ids %v", got)
	}
}

// TestNamesOf 校验按 id 顺序取名, 取不到的 id 直接跳过
func TestNamesOf(t *testing.T) {
	names := map[int64]string{1: "网页默认渠道", 3: "拼多多旗舰店"}
	got := namesOf([]int64{3, 2, 1}, names)
	if len(got) != 2 || got[0] != "拼多多旗舰店" || got[1] != "网页默认渠道" {
		t.Fatalf("unexpected names %v", got)
	}
	if list := namesOf(nil, names); len(list) != 0 {
		t.Fatalf("expect empty names, got %v", list)
	}
}
