package model

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// TimeLayout 是接口与数据库统一使用的时间格式
const TimeLayout = "2006-01-02 15:04:05"

// Time 是按 TimeLayout 输出 json 的时间类型，零值输出空字符串
type Time struct {
	time.Time
}

// MarshalJSON 按 TimeLayout 输出时间
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + t.Format(TimeLayout) + `"`), nil
}

// Scan 实现 sql.Scanner，从数据库读取时间
func (t *Time) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		t.Time = time.Time{}
	case time.Time:
		t.Time = v
	case string:
		return t.parse(v)
	case []byte:
		return t.parse(string(v))
	default:
		return fmt.Errorf("无法把 %T 转换为时间", value)
	}
	return nil
}

// Value 实现 driver.Valuer，把时间写入数据库
func (t Time) Value() (driver.Value, error) {
	if t.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

// Now 返回当前时间
func Now() Time {
	return Time{Time: time.Now()}
}

// parse 解析数据库里的时间文本
func (t *Time) parse(value string) error {
	parsed, err := time.ParseInLocation(TimeLayout, value, time.Local)
	if err != nil {
		return fmt.Errorf("无法解析时间 %q: %w", value, err)
	}
	t.Time = parsed
	return nil
}
