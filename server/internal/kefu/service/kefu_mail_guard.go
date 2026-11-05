package service

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/kefu/mail"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/setting"
)

// 发件人黑名单，命中后不建会话也不自动回复
var mailSenderBlacklist = []string{
	"mailer-daemon@", "postmaster@", "no-reply@", "noreply@", "do-not-reply@",
}

// mailBlockedExtensions 是邮件场景默认拒绝的附件后缀，覆盖可执行文件与脚本
var mailBlockedExtensions = []string{
	".exe", ".bat", ".cmd", ".com", ".scr", ".msi", ".sh", ".js", ".jse",
	".vbs", ".vbe", ".wsf", ".ps1", ".jar", ".reg", ".dll",
}

// 敏感场景关键词，命中后不自动发送，转人工草稿
var mailSensitiveKeywords = []string{
	"投诉", "退款", "退货", "赔偿", "赔付", "法律", "律师", "起诉", "隐私",
	"账号安全", "价格承诺", "退款申请", "complaint", "refund", "lawsuit",
	"legal", "privacy", "chargeback",
}

// mailSkipReason 判断入站邮件是否应该跳过，返回跳过原因，空串表示正常处理
func mailSkipReason(mailbox *model.KefuMailbox, parsed *mail.Parsed) string {
	if parsed.FromAddr == "" {
		return model.KefuMailSkipSenderMissing
	}
	// 自动回复头与群发邮件
	if parsed.IsAutoReply() {
		if parsed.ListID != "" || parsed.Precedence == "bulk" ||
			parsed.Precedence == "list" || parsed.Precedence == "junk" {
			return model.KefuMailSkipBulk
		}
		return model.KefuMailSkipAutoSubmitted
	}
	// 退信与投递状态通知
	if parsed.IsBounce() {
		return model.KefuMailSkipBounce
	}
	// 发件人是本邮箱自身、别名或 user+tag 变体，防自环
	if mailIsSelfSender(mailbox, parsed.FromAddr) {
		return model.KefuMailSkipLoop
	}
	if mailSenderInBlacklist(parsed.FromAddr) {
		return model.KefuMailSkipSpam
	}
	return ""
}

// mailIsSelfSender 判断发件人是不是本邮箱自身、配置的别名或 user+tag 变体，
// 只按本邮箱判断，不能按服务商域名判断，否则 QQ / Gmail 的正常客户会被误判为自环
func mailIsSelfSender(mailbox *model.KefuMailbox, from string) bool {
	from = strings.ToLower(strings.TrimSpace(from))
	own := strings.ToLower(strings.TrimSpace(mailbox.EmailAddress))
	if from == "" || own == "" {
		return false
	}
	if from == own {
		return true
	}
	for _, alias := range strings.Split(mailbox.Aliases, ",") {
		if from == strings.ToLower(strings.TrimSpace(alias)) {
			return true
		}
	}
	at := strings.LastIndex(own, "@")
	if at < 0 {
		return false
	}
	local, domain := own[:at], own[at+1:]
	return strings.HasSuffix(from, "@"+domain) && strings.HasPrefix(from, local+"+")
}

// mailSenderInBlacklist 判断发件人是否命中黑名单
func mailSenderInBlacklist(address string) bool {
	address = strings.ToLower(strings.TrimSpace(address))
	for _, prefix := range mailSenderBlacklist {
		if strings.HasPrefix(address, prefix) {
			return true
		}
	}
	return false
}

// mailAttachmentAllowed 判断邮件附件后缀是否允许，可执行文件与脚本一律拒绝；
// 其它后缀复用配置中心 kefu.file.allowed_ext 的白名单
func mailAttachmentAllowed(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	for _, blocked := range mailBlockedExtensions {
		if ext == blocked {
			return false
		}
	}
	allowed := fileAllowedExt()
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if strings.EqualFold(ext, "."+strings.TrimPrefix(item, ".")) {
			return true
		}
	}
	// 邮件里常见的文档类型即使不在访客附件白名单里也放行
	switch ext {
	case ".eml", ".msg", ".ics", ".csv", ".rtf":
		return true
	default:
		return false
	}
}

// mailReplyLimited 判断是否触发自动回复限流，返回是否受限与中文原因
func mailReplyLimited(
	replyRepository *repository.KefuAiReplyRepository,
	thread *model.KefuMailThread, visitorID int64,
) (bool, string) {
	maxPerThread := setting.GetInt(configKeyMailMaxAutoReplyPerThread,
		defaultMailMaxAutoReplyPerThread)
	if maxPerThread > 0 {
		count, err := replyRepository.CountMailSentByThread(thread.ThreadID)
		if err == nil && count >= int64(maxPerThread) {
			return true, "同一线程自动回复次数已达上限"
		}
	}

	maxPerDay := setting.GetInt(configKeyMailMaxAutoReplyPerSenderDay,
		defaultMailMaxAutoReplyPerSenderDay)
	if maxPerDay > 0 {
		since := model.Time{Time: time.Now().Truncate(24 * time.Hour)}
		count, err := replyRepository.CountMailSentByVisitor(visitorID, since)
		if err == nil && count >= int64(maxPerDay) {
			return true, "同一发件人当日自动回复次数已达上限"
		}
	}

	minInterval := setting.GetInt(configKeyMailMinAutoReplyIntervalMin,
		defaultMailMinAutoReplyIntervalMin)
	if minInterval > 0 {
		latest, err := replyRepository.LatestMailSentTime(thread.ThreadID)
		if err == nil && !latest.IsZero() &&
			time.Since(latest.Time) < time.Duration(minInterval)*time.Minute {
			return true, "距上次自动回复时间过短"
		}
	}
	return false, ""
}

// mailSensitiveHit 判断正文是否命中敏感场景关键词
func mailSensitiveHit(text string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range mailSensitiveKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// mailInWorkTime 判断当前是否在邮箱配置的工作时间内，未开启工作时间时恒为 true
func mailInWorkTime(mailbox *model.KefuMailbox, now time.Time) bool {
	if mailbox.WorkTimeEnabled != 1 {
		return true
	}
	// 节假日优先判断，格式 yyyy-MM-dd
	for _, day := range strings.Split(mailbox.Holidays, ",") {
		if strings.TrimSpace(day) == now.Format("2006-01-02") {
			return false
		}
	}
	if mailbox.WorkDays != "" {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		allowed := false
		for _, item := range strings.Split(mailbox.WorkDays, ",") {
			if strings.TrimSpace(item) == strconv.Itoa(weekday) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	start, ok := parseClock(mailbox.WorkTimeStart)
	if !ok {
		return true
	}
	end, ok := parseClock(mailbox.WorkTimeEnd)
	if !ok {
		return true
	}
	current := now.Hour()*60 + now.Minute()
	if start <= end {
		return current >= start && current <= end
	}
	// 跨天的工作时间，如 22:00-06:00
	return current >= start || current <= end
}

// parseClock 把 HH:MM 转成分钟数
func parseClock(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour := strings.TrimLeft(parts[0], "0")
	minute := strings.TrimLeft(parts[1], "0")
	if hour == "" {
		hour = "0"
	}
	if minute == "" {
		minute = "0"
	}
	h, err := strconv.Atoi(hour)
	if err != nil || h < 0 || h > 23 {
		return 0, false
	}
	m, err := strconv.Atoi(minute)
	if err != nil || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
