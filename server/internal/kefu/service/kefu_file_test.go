package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"admin/server/model"
	"admin/server/pkg/setting"
)

// TestFileAllowedExt 校验后缀白名单解析：去点号、去空项、统一小写
func TestFileAllowedExt(t *testing.T) {
	setting.Replace(map[string]setting.Item{
		configKeyFileAllowedExt: {Value: " JPG , .pdf,,txt"},
	})
	t.Cleanup(func() { setting.Replace(nil) })

	got := fileAllowedExt()
	want := []string{"jpg", "pdf", "txt"}
	if len(got) != len(want) {
		t.Fatalf("unexpected ext list %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected ext list %+v", got)
		}
	}

	s := NewKefuFileService()
	for _, ext := range []string{".jpg", ".pdf", ".txt"} {
		if !s.extAllowed(ext) {
			t.Fatalf("expect %s allowed", ext)
		}
	}
	for _, ext := range []string{".sh", ".exe", ""} {
		if s.extAllowed(ext) {
			t.Fatalf("expect %s rejected", ext)
		}
	}
}

// TestFileUploadDisabled 校验附件开关与大小上限读取的是最新配置
func TestFileUploadDisabled(t *testing.T) {
	setting.Replace(map[string]setting.Item{
		configKeyFileEnabled:   {Value: "false"},
		configKeyFileMaxSizeMB: {Value: "3"},
	})
	t.Cleanup(func() { setting.Replace(nil) })

	s := NewKefuFileService()
	if _, err := s.Upload("127.0.0.1", "a.txt", nil, 1, "text/plain"); !errors.Is(err, ErrFileDisabled) {
		t.Fatalf("expect ErrFileDisabled, got %v", err)
	}
	if got := fileMaxSizeMB(); got != 3 {
		t.Fatalf("unexpected max size %d", got)
	}

	setting.Replace(map[string]setting.Item{
		configKeyFileEnabled:   {Value: "true"},
		configKeyFileMaxSizeMB: {Value: "3"},
	})
	if _, err := s.Upload("127.0.0.1", "a.sh", nil, 1, "text/plain"); !errors.Is(err, ErrFileExtNotAllowed) {
		t.Fatalf("expect ErrFileExtNotAllowed, got %v", err)
	}

	var sizeErr *KefuFileSizeError
	if _, err := s.Upload("127.0.0.1", "a.txt", nil, 4<<20, "text/plain"); !errors.As(err, &sizeErr) {
		t.Fatalf("expect size error, got %v", err)
	}
}

// TestFileRawPath 校验附件读取路径防目录穿越且限制后缀白名单
func TestFileRawPath(t *testing.T) {
	setting.Replace(map[string]setting.Item{
		configKeyFileAllowedExt: {Value: "txt,png"},
	})
	t.Cleanup(func() { setting.Replace(nil) })
	// 上传根目录是包内的相对路径, 切到临时目录避免污染仓库
	t.Chdir(t.TempDir())

	dir := filepath.Join("uploads", "20240101")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	fullPath := filepath.Join(dir, "abc.txt")
	if err := os.WriteFile(fullPath, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	s := NewKefuFileService()
	got, err := s.RawPath("/20240101/abc.txt")
	if err != nil {
		t.Fatalf("expect raw path ok, got %v", err)
	}
	if got != filepath.Join("uploads", "20240101", "abc.txt") {
		t.Fatalf("unexpected raw path %s", got)
	}

	for _, value := range []string{
		"",
		"../secret.txt",
		"/etc/passwd.txt",
		"20240101/../../secret.txt",
		"20240101/abc.sh",
		"20240101/missing.txt",
	} {
		if _, err := s.RawPath(value); err == nil {
			t.Fatalf("expect %q rejected", value)
		}
	}
}

// TestFileIPCounterKey 校验单 IP 日计数按日期分桶，空 ip 用 unknown 兜底
func TestFileIPCounterKey(t *testing.T) {
	want := fileIPCounterKeyPrefix + "1.2.3.4:" + time.Now().Format("20060102")
	if got := fileIPCounterKey(" 1.2.3.4 "); got != want {
		t.Fatalf("unexpected key %s", got)
	}
	want = fileIPCounterKeyPrefix + "unknown:" + time.Now().Format("20060102")
	if got := fileIPCounterKey(""); got != want {
		t.Fatalf("unexpected key %s", got)
	}
}

// TestMessageMeta 校验附件消息的附加信息解析，文本消息与非法 json 返回 nil
func TestMessageMeta(t *testing.T) {
	if got := messageMeta(model.KefuMessageTypeText, `{"fileId":1}`); got != nil {
		t.Fatalf("text message should not carry meta, got %+v", got)
	}
	if got := messageMeta(model.KefuMessageTypeImage, ""); got != nil {
		t.Fatalf("empty payload should not carry meta, got %+v", got)
	}
	if got := messageMeta(model.KefuMessageTypeFile, "not-json"); got != nil {
		t.Fatalf("invalid json should not carry meta, got %+v", got)
	}

	got := messageMeta(model.KefuMessageTypeFile, `{"fileId":3,"name":"a.pdf","size":10}`)
	if got["fileId"] != float64(3) || got["name"] != "a.pdf" {
		t.Fatalf("unexpected meta %+v", got)
	}
}
