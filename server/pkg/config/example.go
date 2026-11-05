package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// exampleTemplate 是配置示例文件的内容，字段与 Config 一一对应，敏感项留空
const exampleTemplate = `# 配置示例文件，由 server -gen-config <path> 生成
# 复制为 config.toml 后按实际环境修改

[server]
addr = ":18080"                  # 监听地址，前端 vite 代理默认指向 18080
mode = "debug"                   # debug 或 release

[log]
level = "debug"                  # 写文件的日志级别
console_level = "info"           # 输出到控制台的日志级别
sql_level = "debug"              # SQL 日志级别，silent 为关闭
path = "logs"                    # 日志目录，为空时只输出到控制台
max_size = 20                    # 单个日志文件的大小上限，单位 MB
max_age = 30                     # 日志文件的保留天数

[db]
dsn = "host=127.0.0.1 port=5432 user=postgres password= dbname=postgres sslmode=disable TimeZone=Asia/Shanghai"
max_idle = 10
max_open = 100
max_lifetime = 3600              # 秒

[redis]
addr = "127.0.0.1:6379"
password = ""
db = 0

[jwt]
secret = ""                      # 必填，后台登录 token 密钥，上线请改成随机长串
user_secret = ""                 # 前台用户 token 密钥，为空时用 secret + "-user"
expire = 7200                    # 秒

[captcha]
length = 4                       # 字符个数
expire = 300                     # 有效期，秒
width = 130                      # 图片宽度，像素
height = 48                      # 图片高度，像素

[security]
config_key = ""                  # 必填（保存密钥类配置时），建议 openssl rand -base64 32 生成；
                                 # 更换后已保存的密钥类配置无法解密，需要重新填写

[wukongim]
api_url = "http://127.0.0.1:5001"  # 服务端 HTTP API 地址，仅内网可达
ws_url = "ws://127.0.0.1:5200"     # 客户端 gateway 地址，登录时下发给前端
webhook_token = ""               # 校验 IM 回调的共享密钥
ai_uid = "ai_service"            # AI 服务号的 UID

[qdrant]
url = "http://127.0.0.1:6334"      # 向量库地址，使用 gRPC 端口
api_key = ""                     # 访问密钥，为空表示未开启鉴权
collection_prefix = "kefu"       # 集合名前缀
vector_size = 1536               # 向量维度，换 embedding 模型后需要重建集合

[cors]
allow_origins = []               # 允许的来源，为空表示不启用跨域（同源反代场景无需配置）
allow_credentials = false        # 是否允许携带 cookie
`

// WriteExample 将配置示例写入 path，文件已存在时覆盖
func WriteExample(path string) error {
	// 自检模板：键名写错或与 Config 结构体不一致时立即失败，避免生成废配置
	decoder := toml.NewDecoder(strings.NewReader(exampleTemplate))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&Config{}); err != nil {
		return fmt.Errorf("配置示例模板不合法: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建目录 %q 失败: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(exampleTemplate), 0o644); err != nil {
		return fmt.Errorf("写入 %q 失败: %w", path, err)
	}
	return nil
}
