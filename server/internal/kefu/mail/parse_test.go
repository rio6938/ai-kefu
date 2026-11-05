package mail

import (
	"encoding/base64"
	"strings"
	"testing"
)

// buildRaw 拼一封多段邮件，正文 base64 编码，附件用 quoted-printable
func buildRaw() []byte {
	text := base64.StdEncoding.EncodeToString([]byte(
		"您好，订单 A123 还没发货。\n\n-- \n客服团队\n\n-----原始邮件-----\n> 引用内容"))
	attachment := base64.StdEncoding.EncodeToString([]byte("hello attachment"))
	raw := "From: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("张三")) + "?= <zhangsan@example.com>\r\n" +
		"To: kefu@example.com\r\n" +
		"Subject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("Re: 订单发货问题")) + "?=\r\n" +
		"Message-ID: <abc@example.com>\r\n" +
		"In-Reply-To: <root@example.com>\r\n" +
		"References: <root@example.com>\r\n" +
		"Date: Mon, 01 Jan 2024 10:00:00 +0800\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mix\"\r\n\r\n" +
		"--mix\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		text + "\r\n" +
		"--mix\r\n" +
		"Content-Type: application/pdf; name=\"invoice.pdf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"Content-Disposition: attachment; filename=\"invoice.pdf\"\r\n\r\n" +
		attachment + "\r\n" +
		"--mix--\r\n"
	return []byte(raw)
}

// TestParse 校验主题解码、引用与签名清洗、附件识别
func TestParse(t *testing.T) {
	parsed, err := Parse(buildRaw())
	if err != nil {
		t.Fatalf("parse mail failed: %v", err)
	}
	if parsed.Subject != "Re: 订单发货问题" {
		t.Fatalf("unexpected subject: %s", parsed.Subject)
	}
	if parsed.SubjectKey != "订单发货问题" {
		t.Fatalf("unexpected subject key: %s", parsed.SubjectKey)
	}
	if parsed.FromAddr != "zhangsan@example.com" || parsed.FromName != "张三" {
		t.Fatalf("unexpected from: %s %s", parsed.FromAddr, parsed.FromName)
	}
	if parsed.MessageID != "abc@example.com" {
		t.Fatalf("unexpected message id: %s", parsed.MessageID)
	}
	if len(parsed.ReferenceList()) != 2 {
		t.Fatalf("unexpected references: %v", parsed.ReferenceList())
	}
	if strings.Contains(parsed.Text, "原始邮件") || strings.Contains(parsed.Text, "客服团队") {
		t.Fatalf("quote or signature not cleaned: %s", parsed.Text)
	}
	if !strings.Contains(parsed.Text, "订单 A123") {
		t.Fatalf("unexpected text: %s", parsed.Text)
	}
	if len(parsed.Attachments) != 1 || parsed.Attachments[0].FileName != "invoice.pdf" {
		t.Fatalf("unexpected attachments: %+v", parsed.Attachments)
	}
	if string(parsed.Attachments[0].Content) != "hello attachment" {
		t.Fatalf("unexpected attachment content: %s", parsed.Attachments[0].Content)
	}
}

// TestHTMLToText 校验 HTML 转纯文本保留段落
func TestHTMLToText(t *testing.T) {
	text := HTMLToText("<p>第一段</p><p>第二段<br/>换行</p><script>bad()</script>")
	if strings.Contains(text, "bad") {
		t.Fatalf("script should be removed: %s", text)
	}
	if !strings.Contains(text, "第一段") || !strings.Contains(text, "换行") {
		t.Fatalf("unexpected text: %s", text)
	}
}

// TestSubjectKey 校验 Re/Fwd/回复 前缀归一化
func TestSubjectKey(t *testing.T) {
	cases := map[string]string{
		"Re: 订单问题":      "订单问题",
		"Fwd: RE: 订单问题": "订单问题",
		"回复：订单问题":       "订单问题",
		"  Order   #1 ": "order #1",
	}
	for input, expected := range cases {
		if got := SubjectKey(input); got != expected {
			t.Fatalf("SubjectKey(%q) = %q, want %q", input, got, expected)
		}
	}
}
