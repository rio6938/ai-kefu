package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	netmail "net/mail"
	"sort"
	"strings"
	"time"
)

// Attachment 是一封邮件的附件
type Attachment struct {
	// 文件名
	FileName string
	// 文件类型, 为空时按 application/octet-stream
	ContentType string
	// 文件内容
	Content []byte
}

// Message 是一封待发送的邮件；
// Body + HTML 是兼容既有调用的简单正文，TextBody + HTMLBody 用于同时发送纯文本与 HTML
type Message struct {
	// 收件人地址
	To []string
	// 抄送地址
	Cc []string
	// 回复地址, 为空时用发件人地址
	ReplyTo string
	// 发件人显示名, 为空时用配置里的显示名
	FromName string
	// 邮件标题
	Subject string
	// 简单正文, HTML 为 false 时作为纯文本, 为 true 时作为 HTML
	Body string
	// Body 是否为 HTML
	HTML bool
	// 纯文本正文
	TextBody string
	// HTML 正文
	HTMLBody string
	// 附件
	Attachments []Attachment
	// 自定义邮件头, 会自动去掉换行防注入
	Headers map[string]string
	// 邮件 Message-ID, 为空时自动生成
	MessageID string
	// In-Reply-To
	InReplyTo string
	// References 引用链
	References []string
}

// validRecipients 校验收件人地址并返回去掉显示名的地址列表
func validRecipients(to []string) ([]string, error) {
	if len(to) == 0 {
		return nil, errors.New("收件人不能为空")
	}
	list := make([]string, 0, len(to))
	for _, addr := range to {
		parsed, err := netmail.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("收件人地址 %q 不正确: %w", addr, err)
		}
		list = append(list, parsed.Address)
	}
	return list, nil
}

// buildMessageWithID 按 RFC 5322 拼装邮件报文，中文标题、文件名与正文做 MIME 编码；
// 同时返回实际使用的 Message-ID，供发信记录回填
func buildMessageWithID(cfg Config, msg Message, to []string) ([]byte, string) {
	fromName := cfg.FromName
	if msg.FromName != "" {
		fromName = msg.FromName
	}
	from := &netmail.Address{Name: fromName, Address: cfg.From}

	replyTo := msg.ReplyTo
	if replyTo == "" {
		replyTo = cfg.From
	}

	textBody := msg.TextBody
	htmlBody := msg.HTMLBody
	if textBody == "" && htmlBody == "" {
		if msg.HTML {
			htmlBody = msg.Body
		} else {
			textBody = msg.Body
		}
	}

	messageID := msg.MessageID
	if messageID == "" {
		messageID = generateMessageID(cfg.From)
	}

	var buf bytes.Buffer
	writeHeader(&buf, "From", from.String())
	writeHeader(&buf, "To", strings.Join(to, ", "))
	if len(msg.Cc) > 0 {
		writeHeader(&buf, "Cc", strings.Join(msg.Cc, ", "))
	}
	writeHeader(&buf, "Reply-To", replyTo)
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	writeHeader(&buf, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "Message-ID", messageID)
	if msg.InReplyTo != "" {
		writeHeader(&buf, "In-Reply-To", ensureAngle(msg.InReplyTo))
	}
	if len(msg.References) > 0 {
		refs := make([]string, 0, len(msg.References))
		for _, item := range msg.References {
			if value := strings.TrimSpace(item); value != "" {
				refs = append(refs, ensureAngle(value))
			}
		}
		writeHeader(&buf, "References", strings.Join(refs, " "))
	}
	// 自定义头按名称排序后写入，保证同一封信的报文稳定，便于排查
	names := make([]string, 0, len(msg.Headers))
	for name := range msg.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writeHeader(&buf, name, msg.Headers[name])
	}

	writeBody(&buf, textBody, htmlBody, msg.Attachments)
	return buf.Bytes(), messageID
}

// writeBody 按正文与附件组合 MIME 结构：
// 有附件用 multipart/mixed 包一层，正文内部再用 multipart/alternative 放纯文本与 HTML
func writeBody(buf *bytes.Buffer, textBody, htmlBody string, attachments []Attachment) {
	hasText := strings.TrimSpace(textBody) != ""
	hasHTML := strings.TrimSpace(htmlBody) != ""
	if !hasText && !hasHTML {
		hasText = true
	}

	if len(attachments) == 0 && !(hasText && hasHTML) {
		// 单段正文，保持与历史实现一致的报文结构
		body := textBody
		if !hasText {
			body = htmlBody
			writeHeader(buf, "Content-Type", "text/html; charset=UTF-8")
		} else {
			writeHeader(buf, "Content-Type", "text/plain; charset=UTF-8")
		}
		writeHeader(buf, "Content-Transfer-Encoding", "base64")
		buf.WriteString("\r\n")
		buf.WriteString(encodeBody(body))
		return
	}

	if len(attachments) == 0 {
		writeHeader(buf, "Content-Type", `multipart/alternative; boundary="`+alternativeBoundary+`"`)
		buf.WriteString("\r\n")
		writeTextPart(buf, textBody, htmlBody)
		return
	}

	writeHeader(buf, "Content-Type", `multipart/mixed; boundary="`+mixedBoundary+`"`)
	buf.WriteString("\r\n")
	buf.WriteString("--" + mixedBoundary + "\r\n")
	writeHeader(buf, "Content-Type", `multipart/alternative; boundary="`+alternativeBoundary+`"`)
	buf.WriteString("\r\n")
	writeTextPart(buf, textBody, htmlBody)
	buf.WriteString("--" + mixedBoundary + "\r\n")

	for _, item := range attachments {
		contentType := strings.TrimSpace(item.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		writeHeader(buf, "Content-Type", contentType+"; name=\""+
			mime.QEncoding.Encode("UTF-8", item.FileName)+"\"")
		writeHeader(buf, "Content-Transfer-Encoding", "base64")
		writeHeader(buf, "Content-Disposition", "attachment; filename=\""+
			mime.QEncoding.Encode("UTF-8", item.FileName)+"\"")
		buf.WriteString("\r\n")
		buf.WriteString(encodeBody(string(item.Content)))
		buf.WriteString("--" + mixedBoundary + "\r\n")
	}
}

// writeTextPart 写入 multipart/alternative 里的纯文本与 HTML 两个分段
func writeTextPart(buf *bytes.Buffer, textBody, htmlBody string) {
	if strings.TrimSpace(textBody) != "" {
		buf.WriteString("--" + alternativeBoundary + "\r\n")
		writeHeader(buf, "Content-Type", "text/plain; charset=UTF-8")
		writeHeader(buf, "Content-Transfer-Encoding", "base64")
		buf.WriteString("\r\n")
		buf.WriteString(encodeBody(textBody))
	}
	if strings.TrimSpace(htmlBody) != "" {
		buf.WriteString("--" + alternativeBoundary + "\r\n")
		writeHeader(buf, "Content-Type", "text/html; charset=UTF-8")
		writeHeader(buf, "Content-Transfer-Encoding", "base64")
		buf.WriteString("\r\n")
		buf.WriteString(encodeBody(htmlBody))
	}
	buf.WriteString("--" + alternativeBoundary + "--\r\n")
}

// writeHeader 写一行邮件头，并去掉值里的换行避免邮件头注入
func writeHeader(buf *bytes.Buffer, name, value string) {
	value = strings.NewReplacer("\r", "", "\n", "").Replace(value)
	buf.WriteString(name)
	buf.WriteString(": ")
	buf.WriteString(value)
	buf.WriteString("\r\n")
}

// encodeBody 把正文 base64 编码并按 76 列折行，避免长行被邮件服务器截断
func encodeBody(body string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	var buf strings.Builder
	for len(encoded) > 76 {
		buf.WriteString(encoded[:76])
		buf.WriteString("\r\n")
		encoded = encoded[76:]
	}
	buf.WriteString(encoded)
	buf.WriteString("\r\n")
	return buf.String()
}

// generateMessageID 生成 <随机串@邮箱域名> 形式的 Message-ID
func generateMessageID(from string) string {
	domain := "localhost"
	if at := strings.LastIndex(from, "@"); at >= 0 && at < len(from)-1 {
		domain = from[at+1:]
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), hex.EncodeToString(random), domain)
}

// ensureAngle 给 Message-ID 补上尖括号
func ensureAngle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "<") {
		return value
	}
	return "<" + value + ">"
}

// 固定的 MIME 分段标识，不引入随机值，便于对比测试与排查
const (
	// mixedBoundary 是 multipart/mixed 的分段标识
	mixedBoundary = "kefu-mail-mixed-boundary"
	// alternativeBoundary 是 multipart/alternative 的分段标识
	alternativeBoundary = "kefu-mail-alternative-boundary"
)

// Build 按发送配置拼装一封邮件的完整报文并返回实际使用的 Message-ID，
// 用于开启「已发送同步」时把同一封邮件 APPEND 到服务商已发送文件夹
func Build(cfg Config, msg Message) ([]byte, string, error) {
	to, err := validRecipients(msg.To)
	if err != nil {
		return nil, "", err
	}
	body, messageID := buildMessageWithID(cfg, msg, to)
	return body, messageID, nil
}
