package repository

import (
	"reflect"
	"testing"

	"admin/server/internal/common"
)

// TestColumnName 校验排序字段白名单, 非白名单字段返回空
func TestColumnName(t *testing.T) {
	cases := map[string]string{
		"lastMessageTime": "last_message_time",
		"messageCount":    "message_count",
		"createTime":      "create_time",
		"docId":           "doc_id",
		"chunkCount":      "chunk_count",
		"fileSize":        "file_size",
		"unknownField":    "",
	}
	for field, want := range cases {
		if got := columnName(field); got != want {
			t.Fatalf("field %q expect %q, got %q", field, want, got)
		}
	}
}

// TestOrderClausesWith 校验默认排序与多字段排序
func TestOrderClausesWith(t *testing.T) {
	defaultOrder := "last_message_time desc"
	if got := orderClausesWith(common.PageParam{}, defaultOrder); !reflect.DeepEqual(got, []string{defaultOrder}) {
		t.Fatalf("unexpected default order %v", got)
	}

	page := common.PageParam{Sort: "messageCount,createTime", Order: "asc,desc"}
	want := []string{"message_count asc", "create_time desc"}
	if got := orderClausesWith(page, defaultOrder); !reflect.DeepEqual(got, want) {
		t.Fatalf("expect %v, got %v", want, got)
	}

	// 非白名单字段被忽略, 全部非法时回到默认排序
	page = common.PageParam{Sort: "unknownField"}
	if got := orderClausesWith(page, defaultOrder); !reflect.DeepEqual(got, []string{defaultOrder}) {
		t.Fatalf("unexpected order for unknown field %v", got)
	}
}
