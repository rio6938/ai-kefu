package mail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Attachment 是解析出的附件
type Attachment struct {
	// 文件名
	FileName string
	// 文件类型
	ContentType string
	// 文件内容
	Content []byte
}

// Parsed 是一封解析后的邮件
type Parsed struct {
	// Message-ID, 去掉尖括号
	MessageID string
	// In-Reply-To, 去掉尖括号
	InReplyTo string
	// References 引用链, 空格分隔, 去掉尖括号
	References string
	// 原始主题
	Subject string
	// 归一化主题
	SubjectKey string
	// 发件人地址
	FromAddr string
	// 发件人名称
	FromName string
	// 收件人地址, 英文逗号分隔
	ToAddr string
	// 抄送地址, 英文逗号分隔
	CcAddr string
	// 邮件时间
	Date time.Time
	// 去掉引用与签名后的正文
	Text string
	// 完整正文, 保留引用与签名, 写入消息 payload 供「显示原文」
	FullText string
	// HTML 正文原文
	HTML string
	// 附件
	Attachments []Attachment
	// Auto-Submitted 头
	AutoSubmitted string
	// Precedence 头
	Precedence string
	// List-Id 头
	ListID string
	// X-Auto-Response-Suppress 头
	AutoResponseSuppress string
	// 原始大小, 字节
	Size int64
	// 退信收件人, 从退信报告里解析, 用于标记访客邮箱无效
	BounceRecipient string
}

// wordDecoder 解码 RFC2047 编码的标题与文件名，同时支持 GBK 等字符集
var wordDecoder = &mime.WordDecoder{
	CharsetReader: func(charsetName string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(charsetName, input)
	},
}

// 引用与签名的分隔标记
var (
	// quoteMarkers 是常见邮件客户端引用原文的分隔行
	quoteMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?im)^-{2,}\s*original message\s*-{2,}\s*$`),
		regexp.MustCompile(`(?im)^-{2,}\s*原始邮件\s*-{2,}\s*$`),
		regexp.MustCompile(`(?im)^-{2,}\s*转发邮件\s*-{2,}\s*$`),
		regexp.MustCompile(`(?im)^on .+wrote:\s*$`),
		regexp.MustCompile(`(?im)^在.+写道[:：]\s*$`),
	}
	// quoteLine 是引用行，> 开头
	quoteLine = regexp.MustCompile(`(?m)^\s*>.*$`)
)

// Parse 解析原始邮件，优先 text/plain，没有时把 HTML 转成文本；
// 解析失败时返回错误，由调用方记录 failed 并继续处理下一封
func Parse(raw []byte) (*Parsed, error) {
	message, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("读取邮件内容失败: %w", err)
	}

	parsed := &Parsed{Size: int64(len(raw))}
	parsed.MessageID = stripAngle(message.Header.Get("Message-ID"))
	parsed.InReplyTo = stripAngle(firstMessageID(message.Header.Get("In-Reply-To")))
	parsed.References = joinMessageIDs(message.Header.Get("References"))
	parsed.Subject = decodeHeader(message.Header.Get("Subject"))
	parsed.SubjectKey = SubjectKey(parsed.Subject)
	parsed.AutoSubmitted = strings.TrimSpace(message.Header.Get("Auto-Submitted"))
	parsed.Precedence = strings.ToLower(strings.TrimSpace(message.Header.Get("Precedence")))
	parsed.ListID = strings.TrimSpace(message.Header.Get("List-Id"))
	parsed.AutoResponseSuppress = strings.TrimSpace(message.Header.Get("X-Auto-Response-Suppress"))
	if addresses := parseAddressList(message.Header.Get("From")); len(addresses) > 0 {
		parsed.FromAddr = strings.ToLower(addresses[0].Address)
		parsed.FromName = decodeHeader(addresses[0].Name)
	}
	parsed.ToAddr = joinAddresses(message.Header.Get("To"))
	parsed.CcAddr = joinAddresses(message.Header.Get("Cc"))
	if date, err := message.Header.Date(); err == nil {
		parsed.Date = date
	}
	if parsed.Date.IsZero() {
		parsed.Date = time.Now()
	}

	body, err := io.ReadAll(message.Body)
	if err != nil {
		return nil, fmt.Errorf("读取邮件正文失败: %w", err)
	}

	var texts, htmls []string
	walkPart(textproto.MIMEHeader(message.Header), body, parsed, &texts, &htmls)

	htmlText := ""
	if len(htmls) > 0 {
		parsed.HTML = strings.Join(htmls, "\n")
		htmlText = HTMLToText(parsed.HTML)
	}
	full := strings.Join(texts, "\n")
	if strings.TrimSpace(full) == "" {
		full = htmlText
	}
	parsed.FullText = strings.TrimSpace(full)
	parsed.Text = CleanBody(parsed.FullText)
	if parsed.Text == "" {
		parsed.Text = parsed.FullText
	}
	return parsed, nil
}

// walkPart 递归遍历 MIME 分段，收集正文与附件
func walkPart(header textproto.MIMEHeader, body []byte, parsed *Parsed,
	texts *[]string, htmls *[]string,
) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain; charset=UTF-8"
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = "text/plain"
		params = map[string]string{}
	}
	mediaType = strings.ToLower(mediaType)

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		for {
			part, err := reader.NextPart()
			if err != nil {
				return
			}
			content, err := io.ReadAll(part)
			if err != nil {
				_ = part.Close()
				return
			}
			_ = part.Close()
			walkPart(part.Header, content, parsed, texts, htmls)
		}
	}

	decoded := decodeTransfer(body, header.Get("Content-Transfer-Encoding"))
	fileName := decodeHeader(params["name"])
	if disposition, dispositionParams, err := mime.ParseMediaType(
		header.Get("Content-Disposition")); err == nil {
		if name := decodeHeader(dispositionParams["filename"]); name != "" {
			fileName = name
		}
		if strings.EqualFold(disposition, "attachment") && fileName == "" {
			fileName = "attachment"
		}
	}

	switch {
	case mediaType == "message/delivery-status":
		if recipient := parseFinalRecipient(string(decoded)); recipient != "" {
			parsed.BounceRecipient = recipient
		}
	case mediaType == "text/plain" && fileName == "":
		*texts = append(*texts, decodeCharset(decoded, params["charset"]))
	case mediaType == "text/html" && fileName == "":
		*htmls = append(*htmls, decodeCharset(decoded, params["charset"]))
	case mediaType == "message/rfc822" && fileName == "":
		if nested, err := netmail.ReadMessage(bytes.NewReader(decoded)); err == nil {
			nestedBody, err := io.ReadAll(nested.Body)
			if err == nil {
				walkPart(textproto.MIMEHeader(nested.Header), nestedBody, parsed, texts, htmls)
			}
		}
	default:
		if fileName == "" {
			return
		}
		parsed.Attachments = append(parsed.Attachments, Attachment{
			FileName:    safeFileName(fileName),
			ContentType: mediaType,
			Content:     decoded,
		})
	}
}

// parseFinalRecipient 从退信报告里取 Final-Recipient，取不到时返回空串
func parseFinalRecipient(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "final-recipient:") {
			continue
		}
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			continue
		}
		return strings.ToLower(strings.TrimSpace(parts[1]))
	}
	return ""
}

// decodeTransfer 解码 Content-Transfer-Encoding
func decodeTransfer(body []byte, encoding string) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		clean := strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(body))
		decoded, err := base64.StdEncoding.DecodeString(clean)
		if err != nil {
			if rawDecoded, rawErr := base64.RawStdEncoding.DecodeString(clean); rawErr == nil {
				return rawDecoded
			}
			return body
		}
		return decoded
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err != nil {
			return body
		}
		return decoded
	default:
		return body
	}
}

// decodeCharset 按字符集把正文转成 UTF-8，字符集为空或未知时原样返回
func decodeCharset(body []byte, charsetName string) string {
	charsetName = strings.TrimSpace(strings.Trim(charsetName, `"'`))
	if charsetName == "" {
		return string(body)
	}
	reader, err := charset.NewReaderLabel(charsetName, bytes.NewReader(body))
	if err != nil {
		return string(body)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(body)
	}
	return string(decoded)
}

// decodeHeader 解码 RFC2047 编码的邮件头
func decodeHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if decoded, err := wordDecoder.DecodeHeader(value); err == nil {
		return strings.TrimSpace(decoded)
	}
	return value
}

// SubjectKey 归一化主题：去掉 Re/Fwd/回复/转发 前缀、压缩空白并转小写
func SubjectKey(subject string) string {
	value := strings.TrimSpace(subject)
	prefixes := []string{"re:", "re：", "re ", "fw:", "fwd:", "回复:", "回复：",
		"答复:", "答复：", "转发:", "转发：", "回覆:", "回覆："}
	for {
		lower := strings.ToLower(value)
		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(lower, prefix) {
				value = strings.TrimSpace(value[len(prefix):])
				matched = true
				break
			}
		}
		if !matched {
			break
		}
	}
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

// CleanBody 去掉引用原文与签名，只保留客户本次输入的内容
func CleanBody(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	text := value
	// 按引用分隔行截断，取最早出现的标记
	cut := len(text)
	for _, marker := range quoteMarkers {
		if loc := marker.FindStringIndex(text); loc != nil && loc[0] < cut {
			cut = loc[0]
		}
	}
	if loc := signatureIndex(text); loc >= 0 && loc < cut {
		cut = loc
	}
	text = text[:cut]
	// 去掉零散的 > 引用行
	text = quoteLine.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

// signatureIndex 返回签名分隔行（-- 加空格）的位置，找不到返回 -1
func signatureIndex(value string) int {
	lines := strings.Split(value, "\n")
	offset := 0
	for _, line := range lines {
		if strings.TrimRight(line, " \r") == "--" {
			return offset
		}
		offset += len(line) + 1
	}
	return -1
}

// HTMLToText 把 HTML 转成纯文本，保留段落换行，去掉脚本与样式
func HTMLToText(value string) string {
	doc, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return value
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "script", "style", "head", "title", "meta":
				return
			case "br":
				builder.WriteString("\n")
				return
			}
			if isBlockElement(node.Data) {
				builder.WriteString("\n")
			}
		}
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if node.Type == html.ElementNode && isBlockElement(node.Data) {
			builder.WriteString("\n")
		}
	}
	walk(doc)

	lines := strings.Split(builder.String(), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// isBlockElement 判断 HTML 标签是否是块级元素，用于补换行
func isBlockElement(name string) bool {
	switch strings.ToLower(name) {
	case "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "section", "article", "table", "ul", "ol", "pre":
		return true
	default:
		return false
	}
}

// safeFileName 去掉文件名里的路径分隔符，避免落盘时目录穿越
func safeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "attachment"
	}
	if len([]rune(name)) > 120 {
		return string([]rune(name)[:120])
	}
	return name
}

// stripAngle 去掉 Message-ID 两侧的尖括号
func stripAngle(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSuffix(value, ">")
	return strings.TrimSpace(value)
}

// firstMessageID 从可能包含多个 id 的头里取第一个
func firstMessageID(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// joinMessageIDs 把 References 头里的多个 id 去掉尖括号后拼接
func joinMessageIDs(value string) string {
	fields := strings.Fields(value)
	list := make([]string, 0, len(fields))
	for _, item := range fields {
		if id := stripAngle(item); id != "" {
			list = append(list, id)
		}
	}
	return strings.Join(list, " ")
}

// parseAddressList 解析邮件地址列表，解析失败时返回空
func parseAddressList(value string) []*netmail.Address {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	list, err := netmail.ParseAddressList(value)
	if err != nil {
		if address, addrErr := netmail.ParseAddress(value); addrErr == nil {
			return []*netmail.Address{address}
		}
		return nil
	}
	return list
}

// joinAddresses 把地址列表转成英文逗号分隔的纯地址
func joinAddresses(value string) string {
	list := parseAddressList(value)
	addresses := make([]string, 0, len(list))
	for _, item := range list {
		addresses = append(addresses, strings.ToLower(item.Address))
	}
	return strings.Join(addresses, ",")
}

// IsAutoReply 判断邮件是否是自动回复或群发，命中后不建会话也不触发 AI
func (p *Parsed) IsAutoReply() bool {
	if p.AutoSubmitted != "" && !strings.EqualFold(p.AutoSubmitted, "no") {
		return true
	}
	switch p.Precedence {
	case "bulk", "junk", "list":
		return true
	}
	if p.ListID != "" || p.AutoResponseSuppress != "" {
		return true
	}
	// 部分客户端不带 Auto-Submitted，按主题关键字兜底识别自动回复
	subject := strings.ToLower(p.Subject)
	for _, keyword := range []string{"自动回复", "auto reply", "automatic reply",
		"out of office", "不在办公室"} {
		if strings.Contains(subject, keyword) {
			return true
		}
	}
	return false
}

// IsBounce 判断邮件是否是退信或投递状态通知
func (p *Parsed) IsBounce() bool {
	subject := strings.ToLower(p.Subject)
	for _, keyword := range []string{"undeliverable", "delivery status notification",
		"delivery failure", "mail delivery failed", "退信", "投递失败"} {
		if strings.Contains(subject, keyword) {
			return true
		}
	}
	for _, prefix := range []string{"mailer-daemon@", "postmaster@"} {
		if strings.HasPrefix(p.FromAddr, prefix) {
			return true
		}
	}
	return false
}

// ReferenceList 返回可用于线程匹配的引用链，末尾追加 In-Reply-To
func (p *Parsed) ReferenceList() []string {
	list := make([]string, 0, 3)
	if p.References != "" {
		list = append(list, strings.Fields(p.References)...)
	}
	if p.InReplyTo != "" {
		list = append(list, p.InReplyTo)
	}
	return list
}
