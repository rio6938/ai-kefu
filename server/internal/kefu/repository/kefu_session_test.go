package repository

import (
	"testing"

	"admin/server/model"
)

// TestIsUnassignedStatus 校验只有待接待与 AI 进行中的会话不做客服归属过滤，
// 其余状态仍按接待客服过滤
func TestIsUnassignedStatus(t *testing.T) {
	queueing := model.KefuSessionStatusQueueing
	ai := model.KefuSessionStatusAi
	human := model.KefuSessionStatusHuman
	cases := []struct {
		name   string
		status *int
		want   bool
	}{
		{"待接待", &queueing, true},
		{"AI 进行中", &ai, true},
		{"人工进行中", &human, false},
		{"未指定状态", nil, false},
	}
	for _, item := range cases {
		if got := isUnassignedStatus(item.status); got != item.want {
			t.Fatalf("%s: expect %v, got %v", item.name, item.want, got)
		}
	}
}
