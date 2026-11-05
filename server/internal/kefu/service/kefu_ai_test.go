package service

import (
	"strings"
	"testing"
)

// TestReplacePromptVariables 校验模板变量替换: 已知变量替换, 未知变量原样保留
func TestReplacePromptVariables(t *testing.T) {
	values := map[string]string{
		"channelName": "官网",
		"visitorName": "访客001",
		"agentName":   "小美",
		"knowledge":   "",
		"dialogues":   "示例对话",
		"nowTime":     "2025-01-01 00:00:00",
	}
	text := "{channelName}/{visitorName}/{agentName}/{dialogues}/{unknown}"
	got := replacePromptVariables(text, values)
	if strings.Contains(got, "{channelName}") || !strings.Contains(got, "官网/访客001/小美/示例对话") {
		t.Fatalf("unexpected prompt %q", got)
	}
	if !strings.Contains(got, "{unknown}") {
		t.Fatalf("unknown variable should be kept, got %q", got)
	}
}

// TestRenderPromptVariables 校验模板里用到的变量会被记录
func TestRenderPromptVariables(t *testing.T) {
	if got := renderPromptVariables("你好"); got != "[]" {
		t.Fatalf("expect empty variables, got %q", got)
	}
	got := renderPromptVariables("你是{channelName}客服，当前时间{nowTime}")
	if got != `["channelName","nowTime"]` {
		t.Fatalf("unexpected variables %q", got)
	}
}

// TestJoinPrompt 校验系统提示词与角色提示词的空值拼接
func TestJoinPrompt(t *testing.T) {
	if got := joinPrompt("a", ""); got != "a" {
		t.Fatalf("unexpected prompt %q", got)
	}
	if got := joinPrompt("", "b"); got != "b" {
		t.Fatalf("unexpected prompt %q", got)
	}
	if got := joinPrompt("a", "b"); got != "a\n\nb" {
		t.Fatalf("unexpected prompt %q", got)
	}
}

// TestContainsSensitiveWord 校验敏感词命中判断, 空词与空文本都不算命中
func TestContainsSensitiveWord(t *testing.T) {
	words := []string{"免运费", " ", ""}
	if !containsSensitiveWord("我们承诺免运费", words) {
		t.Fatal("expect sensitive word matched")
	}
	if containsSensitiveWord("我们承诺包邮", words) {
		t.Fatal("expect no sensitive word matched")
	}
	if containsSensitiveWord("", words) {
		t.Fatal("empty text should not match")
	}
}

// TestHashApiKey 校验密钥哈希稳定且不可逆
func TestHashApiKey(t *testing.T) {
	first := hashApiKey("kefu_sk_demo")
	if first != hashApiKey("kefu_sk_demo") {
		t.Fatal("hash should be stable")
	}
	if first == hashApiKey("kefu_sk_demo2") || len(first) != 64 {
		t.Fatalf("unexpected hash %q", first)
	}
}

// TestParseApiKeyExpireTime 校验过期时间: 空串表示永不过期, 非法格式返回业务错误
func TestParseApiKeyExpireTime(t *testing.T) {
	expire, err := parseApiKeyExpireTime("")
	if err != nil || !expire.IsZero() {
		t.Fatalf("empty expire time should be zero, err=%v", err)
	}
	if _, err := parseApiKeyExpireTime("2025-01-01 00:00:00"); err != nil {
		t.Fatalf("parse expire time: %v", err)
	}
	if _, err := parseApiKeyExpireTime("2025/01/01"); err == nil {
		t.Fatal("expect error for invalid expire time")
	}
}
