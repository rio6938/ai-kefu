package service

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"

	"admin/server/model"
	"admin/server/pkg/log"
	"go.uber.org/zap"
)

// 邮件模板类型
const (
	// mailTemplateAck 是首次来信自动确认
	mailTemplateAck = "ack"
	// mailTemplateOffHours 是非工作时间提示
	mailTemplateOffHours = "offHours"
	// mailTemplateHandoff 是转人工告知
	mailTemplateHandoff = "handoff"
	// mailTemplateClosed 是会话关闭告知
	mailTemplateClosed = "closed"
)

// mailPlaceholder 匹配模板里未替换的占位符，渲染后统一去掉
var mailPlaceholder = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// defaultMailTemplates 返回内置的中英模板，邮箱未配置时使用
func defaultMailTemplates() model.KefuMailTemplates {
	return model.KefuMailTemplates{
		Ack: model.KefuMailTemplatePair{
			Zh: "您好 {{senderName}}：\n\n已收到您的邮件（会话编号 {{sessionNo}}），我们会在服务时间内尽快回复您。\n\n主题：{{subject}}\n服务时间：{{workTime}}\n\n{{company}}",
			En: "Hello {{senderName}},\n\nWe have received your email (ticket {{sessionNo}}) and will reply as soon as possible during business hours.\n\nSubject: {{subject}}\nBusiness hours: {{workTime}}\n\n{{company}}",
		},
		OffHours: model.KefuMailTemplatePair{
			Zh: "您好 {{senderName}}：\n\n您的邮件已收到，当前是非工作时间，我们会在下一个工作时间尽快回复。\n\n服务时间：{{workTime}}\n\n{{company}}",
			En: "Hello {{senderName}},\n\nYour email has been received. It is currently outside business hours, and we will reply on the next business day.\n\nBusiness hours: {{workTime}}\n\n{{company}}",
		},
		Handoff: model.KefuMailTemplatePair{
			Zh: "您好 {{senderName}}：\n\n您的问题已转交人工客服处理，我们会在服务时间内尽快回复。\n\n会话编号：{{sessionNo}}\n\n{{company}}",
			En: "Hello {{senderName}},\n\nYour request has been handed over to our support team, and we will reply during business hours.\n\nTicket: {{sessionNo}}\n\n{{company}}",
		},
		Closed: model.KefuMailTemplatePair{
			Zh: "您好 {{senderName}}：\n\n本次会话已结束（会话编号 {{sessionNo}}）。如仍有问题，直接回复本邮件即可继续咨询。\n\n{{company}}",
			En: "Hello {{senderName}},\n\nThis conversation has been closed (ticket {{sessionNo}}). If you still need help, simply reply to this email.\n\n{{company}}",
		},
	}
}

// parseMailTemplates 解析邮箱配置里的模板 JSON，为空或格式不正确时用内置模板补齐
func parseMailTemplates(mailbox *model.KefuMailbox) model.KefuMailTemplates {
	defaults := defaultMailTemplates()
	if mailbox == nil || strings.TrimSpace(mailbox.Templates) == "" {
		return defaults
	}
	parsed := new(model.KefuMailTemplates)
	if err := json.Unmarshal([]byte(mailbox.Templates), parsed); err != nil {
		log.Logger.Warn("decode kefu mail templates failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		return defaults
	}
	return model.KefuMailTemplates{
		Ack:      fillTemplatePair(parsed.Ack, defaults.Ack),
		OffHours: fillTemplatePair(parsed.OffHours, defaults.OffHours),
		Handoff:  fillTemplatePair(parsed.Handoff, defaults.Handoff),
		Closed:   fillTemplatePair(parsed.Closed, defaults.Closed),
	}
}

// fillTemplatePair 用默认模板补齐缺失的语言
func fillTemplatePair(
	value model.KefuMailTemplatePair, fallback model.KefuMailTemplatePair,
) model.KefuMailTemplatePair {
	if strings.TrimSpace(value.Zh) == "" {
		value.Zh = fallback.Zh
	}
	if strings.TrimSpace(value.En) == "" {
		value.En = fallback.En
	}
	return value
}

// mailTemplateOf 按类型与语言取模板
func mailTemplateOf(mailbox *model.KefuMailbox, kind, language string) string {
	templates := parseMailTemplates(mailbox)
	var pair model.KefuMailTemplatePair
	switch kind {
	case mailTemplateAck:
		pair = templates.Ack
	case mailTemplateOffHours:
		pair = templates.OffHours
	case mailTemplateHandoff:
		pair = templates.Handoff
	case mailTemplateClosed:
		pair = templates.Closed
	default:
		pair = templates.Ack
	}
	if language == model.KefuMailLanguageEn {
		return pair.En
	}
	return pair.Zh
}

// renderMailTemplate 替换模板变量，未提供的变量替换成空串，不留下原始占位符
func renderMailTemplate(template string, values map[string]string) string {
	rendered := template
	for key, value := range values {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(mailPlaceholder.ReplaceAllString(rendered, ""))
}

// mailLanguageByText 按正文 CJK 字符占比判断语言，用于 language = auto 的场景
func mailLanguageByText(text string) string {
	cjk, total := 0, 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if unicode.Is(unicode.Han, r) {
			cjk++
		}
	}
	if total == 0 {
		return model.KefuMailLanguageZh
	}
	if float64(cjk)/float64(total) >= 0.3 {
		return model.KefuMailLanguageZh
	}
	return model.KefuMailLanguageEn
}

// mailReplyLanguage 返回本次回复使用的语言：邮箱固定语言优先，否则按来信判断
func mailReplyLanguage(mailbox *model.KefuMailbox, text string) string {
	language := mailboxLanguage(mailbox)
	if language == model.KefuMailLanguageZh || language == model.KefuMailLanguageEn {
		return language
	}
	return mailLanguageByText(text)
}

// mailSignatureOf 返回对应语言的签名，签名里的 \n 原样保留
func mailSignatureOf(mailbox *model.KefuMailbox, language string) string {
	if mailbox == nil {
		return ""
	}
	if language == model.KefuMailLanguageEn {
		return strings.TrimSpace(mailbox.SignatureEn)
	}
	return strings.TrimSpace(mailbox.SignatureZh)
}

// mailPlainText 拼装最终发送的纯文本正文：正文 + 签名 + 可选原文引用
func mailPlainText(body, signature, quote string) string {
	parts := make([]string, 0, 3)
	if value := strings.TrimSpace(body); value != "" {
		parts = append(parts, value)
	}
	if signature != "" {
		parts = append(parts, signature)
	}
	if value := strings.TrimSpace(quote); value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n\n")
}

// mailQuoteOriginal 生成回复里的原文引用，语言跟随回复语言
func mailQuoteOriginal(original, language string) string {
	original = strings.TrimSpace(original)
	if original == "" {
		return ""
	}
	header := "----- 原始邮件 -----"
	if language == model.KefuMailLanguageEn {
		header = "----- Original Message -----"
	}
	lines := strings.Split(original, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = "> " + line
	}
	return header + "\n" + strings.Join(lines, "\n")
}

// mailTextToHTML 把纯文本转成简单 HTML：段落转 p，段内换行转 br，特殊字符转义
func mailTextToHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	blocks := strings.Split(text, "\n\n")
	htmlBlocks := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		escaped := html.EscapeString(block)
		escaped = strings.ReplaceAll(escaped, "\n", "<br/>")
		htmlBlocks = append(htmlBlocks, "<p>"+escaped+"</p>")
	}
	return strings.Join(htmlBlocks, "\n")
}

// mailTemplateVars 组装模板变量，取不到的字段用空串或默认文案
func mailTemplateVars(
	mailbox *model.KefuMailbox, senderName, subject, sessionNo, agentName, company string,
) map[string]string {
	if strings.TrimSpace(senderName) == "" {
		senderName = mailbox.EmailAddress
	}
	workTime := strings.TrimSpace(mailbox.WorkTimeStart)
	if workTime != "" && strings.TrimSpace(mailbox.WorkTimeEnd) != "" {
		workTime = mailbox.WorkTimeStart + "-" + mailbox.WorkTimeEnd
	}
	if workTime == "" {
		workTime = "9:00-18:00"
	}
	return map[string]string{
		"senderName": senderName,
		"subject":    subject,
		"sessionNo":  sessionNo,
		"agentName":  agentName,
		"company":    company,
		"date":       time.Now().Format("2006-01-02"),
		"workTime":   workTime,
	}
}
