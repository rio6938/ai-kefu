package repository

import (
	"strings"

	"admin/server/internal/common"
)

// orderClauses 返回分页排序条件，未指定排序时使用默认排序
func orderClauses(page common.PageParam, defaultOrder string) []string {
	return orderClausesWith(page, defaultOrder)
}

// orderClausesWith 返回分页排序条件，支持前端传多个字段与排序方式
func orderClausesWith(page common.PageParam, defaultOrder string) []string {
	if page.Sort == "" {
		return []string{defaultOrder}
	}

	fields := strings.Split(page.Sort, ",")
	orders := strings.Split(page.Order, ",")
	clauses := make([]string, 0, len(fields))
	for i, field := range fields {
		column := columnName(strings.TrimSpace(field))
		if column == "" {
			continue
		}

		// order 数量不足时沿用一个 order，保持与单字段排序一致的表现
		order := orders[len(orders)-1]
		if i < len(orders) {
			order = orders[i]
		}
		if strings.EqualFold(strings.TrimSpace(order), "desc") {
			order = "desc"
		} else {
			order = "asc"
		}
		clauses = append(clauses, column+" "+order)
	}
	if len(clauses) == 0 {
		return []string{defaultOrder}
	}
	return clauses
}

// columnName 把前端字段名转换为客服模块的数据库列名，非白名单字段返回空
func columnName(field string) string {
	switch field {
	case "channelId":
		return "channel_id"
	case "channelCode":
		return "channel_code"
	case "channelType":
		return "channel_type"
	case "needLogin":
		return "need_login"
	case "agentId":
		return "agent_id"
	case "agentNo":
		return "agent_no"
	case "sessionId":
		return "session_id"
	case "sessionNo":
		return "session_no"
	case "visitorId":
		return "visitor_id"
	case "aiMode":
		return "ai_mode"
	case "aiFailed":
		return "ai_failed"
	case "queueTime":
		return "queue_time"
	case "startTime":
		return "start_time"
	case "endTime":
		return "end_time"
	case "lastMessageTime":
		return "last_message_time"
	case "messageCount":
		return "message_count"
	case "unreadCount":
		return "unread_count"
	case "messageId":
		return "message_id"
	case "docId":
		return "doc_id"
	case "chunkCount":
		return "chunk_count"
	case "fileSize":
		return "file_size"
	case "clientMsgNo":
		return "client_msg_no"
	case "fromRole":
		return "from_role"
	case "contentType":
		return "content_type"
	case "aiGenerated":
		return "ai_generated"
	case "sortNumber":
		return "sort_number"
	case "status", "deleted", "name":
		return field
	case "createTime":
		return "create_time"
	case "updateTime":
		return "update_time"
	default:
		return ""
	}
}
