package service

import (
	"testing"
	"time"

	"admin/server/model"
)

// TestSortAgentsForAssign 校验排序规则: 会话数升序, 最近分配升序, 客服id升序
func TestSortAgentsForAssign(t *testing.T) {
	agents := []*model.KefuAgent{
		{AgentID: 3},
		{AgentID: 2},
		{AgentID: 1},
		{AgentID: 4},
	}
	loads := map[int64]int64{1: 2, 2: 0, 3: 0, 4: 1}
	lastAssign := map[int64]model.Time{
		2: {Time: timeOf("2024-01-02 10:00:00")},
		3: {Time: timeOf("2024-01-01 10:00:00")},
	}

	sortAgentsForAssign(agents, loads, lastAssign)

	want := []int64{3, 2, 4, 1}
	for i, agentID := range want {
		if agents[i].AgentID != agentID {
			t.Fatalf("index %d: expect agent %d, got %d", i, agentID, agents[i].AgentID)
		}
	}
}

// TestMatchTransferKeyword 校验转接关键词匹配，空消息与空词库都不命中
func TestMatchTransferKeyword(t *testing.T) {
	keywords := []string{"退款", "发票", " "}
	if got := matchTransferKeyword("我要申请退款", keywords); got != "退款" {
		t.Fatalf("expect 退款, got %q", got)
	}
	if got := matchTransferKeyword("你好", keywords); got != "" {
		t.Fatalf("expect no hit, got %q", got)
	}
	if got := matchTransferKeyword("", keywords); got != "" {
		t.Fatalf("expect no hit for empty content, got %q", got)
	}
	if got := matchTransferKeyword("退款", nil); got != "" {
		t.Fatalf("expect no hit for empty keywords, got %q", got)
	}
}

// TestAgentUserID 校验从 IM 用户标识解析客服的管理员用户id
func TestAgentUserID(t *testing.T) {
	cases := map[string]int64{
		"agent_12":   12,
		"agent_0":    0,
		"agent_abc":  0,
		"visitor_12": 0,
		"ai_service": 0,
		"agent_":     0,
	}
	for uid, want := range cases {
		got, ok := agentUserID(uid)
		if want == 0 {
			if ok {
				t.Fatalf("uid %q expect invalid, got %d", uid, got)
			}
			continue
		}
		if !ok || got != want {
			t.Fatalf("uid %q expect %d, got %d", uid, want, got)
		}
	}
}

// TestMaxConcurrentOf 校验未配置并发上限时使用默认值
func TestMaxConcurrentOf(t *testing.T) {
	if got := maxConcurrentOf(nil); got != defaultAgentMaxConcurrent {
		t.Fatalf("expect default, got %d", got)
	}
	if got := maxConcurrentOf(&model.KefuAgent{MaxConcurrent: 3}); got != 3 {
		t.Fatalf("expect 3, got %d", got)
	}
}

// timeOf 解析固定时间文本，测试排序时使用
func timeOf(value string) time.Time {
	parsed, err := time.ParseInLocation(model.TimeLayout, value, time.Local)
	if err != nil {
		panic(err)
	}
	return parsed
}
