package captcha

import (
	"image/color"
	"time"

	"github.com/mojocn/base64Captcha"
	goredis "github.com/redis/go-redis/v9"

	"admin/server/pkg/config"
)

// showLineOptions 是验证码的干扰线，保持清爽易辨认
const showLineOptions = base64Captcha.OptionShowSineLine

// background 是验证码背景色，使用全透明背景，
// 避免随机浅色背景与前端容器底色不一致而在两侧露出白边
var background = &color.RGBA{}

// Captcha 是全局验证码实例，Init 之后可用
var Captcha *base64Captcha.Captcha

// debug 为 true 时把验证码文本一起返回，方便开发环境调试
var debug bool

// Result 是获取验证码接口的返回结构
type Result struct {
	// Base64 是图形验证码 base64 数据
	Base64 string `json:"base64"`
	// CaptchaKey 是图形验证码 key
	CaptchaKey string `json:"captchaKey"`
	// Text 是开发环境返回的验证码文本，生产环境为空
	Text string `json:"text"`
}

// Init 用 redis 作为存储初始化验证码
func Init(cfg config.Captcha, isDebug bool, client *goredis.Client) {
	driver := base64Captcha.NewDriverString(
		cfg.Height,
		cfg.Width,
		0,
		showLineOptions,
		cfg.Length,
		base64Captcha.TxtAlphabet,
		background,
		nil,
		nil,
	)
	Captcha = base64Captcha.NewCaptcha(driver, &store{
		client: client,
		expire: time.Duration(cfg.Expire) * time.Second,
	})
	debug = isDebug
}

// Generate 生成验证码，开发环境会同时返回验证码文本
func Generate() (*Result, error) {
	id, base64Data, answer, err := Captcha.Generate()
	if err != nil {
		return nil, err
	}

	result := &Result{Base64: base64Data, CaptchaKey: id}
	if debug {
		result.Text = answer
	}
	return result, nil
}

// Verify 校验验证码，验证码只能用一次
func Verify(id, answer string) bool {
	return Captcha.Verify(id, answer, true)
}
