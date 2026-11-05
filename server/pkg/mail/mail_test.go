package mail

import (
	"strings"
	"testing"
)

// TestBuildMessage 校验 MIME 头、线程头与附件编码
func TestBuildMessage(t *testing.T) {
	cfg := Config{
		Host:     "smtp.example.com",
		Port:     465,
		SSL:      true,
		Username: "kefu@example.com",
		From:     "kefu@example.com",
		FromName: "客服团队",
	}
	raw, messageID, err := Build(cfg, Message{
		To:          []string{"customer@example.com"},
		Subject:     "Re: 订单问题",
		TextBody:    "您好，已为您处理。",
		HTMLBody:    "<p>您好，已为您处理。</p>",
		InReplyTo:   "root@example.com",
		References:  []string{"root@example.com"},
		Headers:     map[string]string{"Auto-Submitted": "auto-replied"},
		Attachments: []Attachment{{FileName: "发票.pdf", ContentType: "application/pdf", Content: []byte("pdf")}},
	})
	if err != nil {
		t.Fatalf("build mail failed: %v", err)
	}
	if messageID == "" || !strings.HasPrefix(messageID, "<") || !strings.HasSuffix(messageID, ">") {
		t.Fatalf("unexpected message id: %s", messageID)
	}

	body := string(raw)
	for _, expected := range []string{
		"multipart/mixed",
		"multipart/alternative",
		"In-Reply-To: <root@example.com>",
		"References: <root@example.com>",
		"Auto-Submitted: auto-replied",
		"Content-Disposition: attachment",
		"MIME-Version: 1.0",
		"Date:",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in message", expected)
		}
	}
	if strings.Contains(body, "Subject: Re: 订单问题") {
		t.Fatalf("subject should be encoded")
	}
}

// TestSendWithoutRecipient 校验收件人为空时返回错误
func TestSendWithoutRecipient(t *testing.T) {
	client, err := New(Config{
		Host: "smtp.example.com", Port: 465, From: "kefu@example.com",
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if err := client.Send(Message{Subject: "test"}); err == nil {
		t.Fatalf("expected error when recipient is empty")
	}
}
