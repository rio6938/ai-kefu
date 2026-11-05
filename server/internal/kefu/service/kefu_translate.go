package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"

	"go.uber.org/zap"
)

// 翻译能力使用到的配置项，全部可在配置中心修改，改完立即生效
const (
	// configKeyTranslateEnabled 是翻译能力的总开关
	configKeyTranslateEnabled = "kefu.translate.enabled"
	// configKeyTranslateTargetLanguage 是默认的目标语言
	configKeyTranslateTargetLanguage = "kefu.translate.target_language"
	// configKeyTranslateTimeoutMs 是单次翻译的超时时间, 单位毫秒
	configKeyTranslateTimeoutMs = "kefu.translate.timeout_ms"
)

// 翻译使用到的其它常量
const (
	// defaultTranslateTargetLanguage 是目标语言的默认值
	defaultTranslateTargetLanguage = "zh-CN"
	// defaultTranslateTimeoutMs 是翻译超时的默认值
	defaultTranslateTimeoutMs = 15000
	// translateMaxTextLength 是单次翻译的文本长度上限, 单位字符
	translateMaxTextLength = 2000
	// translateMaxTokens 是译文的最大 token 数
	translateMaxTokens = 1024
	// translateSystemPrompt 是翻译的系统提示词，固定只输出译文：
	// 保留原格式，不改写专有名词、链接与表情，避免译文里混进解释
	translateSystemPrompt = "你是专业翻译。把用户发来的内容翻译成目标语言，只输出译文，" +
		"保留原有的换行、列表与标点等格式，专有名词、代码块、链接与表情保持原样，" +
		"不要解释、不要加引号、不要输出任何前后缀。"
)

// 翻译相关的业务错误
var (
	// ErrTranslateDisabled 是翻译能力已关闭
	ErrTranslateDisabled = errors.New("翻译功能已关闭")
	// ErrTranslateModelMissing 是还没有可用模型
	ErrTranslateModelMissing = repository.ErrModelNotFound
	// ErrTranslateTextRequired 是待翻译的文本为空
	ErrTranslateTextRequired = errors.New("请先输入需要翻译的内容")
	// ErrTranslateTextTooLong 是待翻译的文本超长
	ErrTranslateTextTooLong = errors.New("翻译内容不能超过 2000 字")
	// ErrTranslateFailed 是模型没有返回译文
	ErrTranslateFailed = errors.New("翻译失败，请稍后重试")
)

// KefuTranslateService 是客服控制台翻译的业务处理：
// 客户消息自动翻译与输入框翻译按钮共用，
// 模型复用已配置的对话模型，不单独引入翻译服务
type KefuTranslateService struct {
	// 模型配置的数据访问
	modelRepository *repository.KefuModelRepository
}

// NewKefuTranslateService 创建翻译的业务处理
func NewKefuTranslateService() *KefuTranslateService {
	return &KefuTranslateService{
		modelRepository: repository.NewKefuModelRepository(),
	}
}

// Translate 把文本翻译成目标语言，source 留空时由模型自动识别；
// 返回的错误都是中文提示，调用方按错误类型映射响应
func (s *KefuTranslateService) Translate(
	ctx context.Context, form model.KefuTranslateForm,
) (*model.KefuTranslateResult, error) {
	if !setting.GetBool(configKeyTranslateEnabled, true) {
		return nil, ErrTranslateDisabled
	}

	text := strings.TrimSpace(form.Text)
	if text == "" {
		return nil, ErrTranslateTextRequired
	}
	if utf8.RuneCountInString(text) > translateMaxTextLength {
		return nil, ErrTranslateTextTooLong
	}

	source := strings.TrimSpace(form.Source)
	target := strings.TrimSpace(form.Target)
	if target == "" {
		target = translateTargetLanguage()
	}

	record, err := s.pickTranslateModel()
	if err != nil {
		return nil, err
	}
	options, err := aiclient.FromModel(record)
	if err != nil {
		return nil, err
	}
	client, err := aiclient.New(options)
	if err != nil {
		return nil, err
	}

	// 翻译要求确定性输出，温度固定为 0
	req := client.Request([]aiclient.Message{
		{Role: aiclient.RoleSystem, Content: translateSystemPrompt},
		{Role: aiclient.RoleUser, Content: translateUserPrompt(text, source, target)},
	})
	req.Temperature = 0
	req.MaxTokens = translateMaxTokens

	timeout := time.Duration(setting.GetInt(configKeyTranslateTimeoutMs, defaultTranslateTimeoutMs)) * time.Millisecond
	translateCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	resp, err := client.ChatOnce(translateCtx, req)
	costMs := time.Since(started).Milliseconds()
	if err != nil {
		log.Logger.Warn("translate kefu message failed",
			zap.Int64("modelId", record.ModelID), zap.String("target", target),
			zap.Int("chars", utf8.RuneCountInString(text)),
			zap.Int64("costMs", costMs), zap.Error(err))
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, ErrTranslateFailed
	}

	translated := strings.TrimSpace(resp.Choices[0].Message.Content)
	if translated == "" {
		return nil, ErrTranslateFailed
	}
	// 上游返回的用量没有值时按字符数估算，便于排查额度消耗
	log.Logger.Info("translate kefu message",
		zap.Int64("modelId", record.ModelID),
		zap.String("model", record.Model),
		zap.String("source", source), zap.String("target", target),
		zap.Int("chars", utf8.RuneCountInString(text)),
		zap.Int64("costMs", costMs),
		zap.Int("totalTokens", resp.Usage.TotalTokens))

	return &model.KefuTranslateResult{
		Text:   translated,
		Source: source,
		Target: target,
		Model:  record.Model,
	}, nil
}

// pickTranslateModel 按顺序挑翻译用的模型：
// 先取勾选了「用作翻译模型」的启用模型（多条取 model_id 最小），
// 没有勾选时回退默认对话模型，都没有返回 ErrTranslateModelMissing
func (s *KefuTranslateService) pickTranslateModel() (*model.KefuModel, error) {
	record, err := s.modelRepository.GetTranslateModel()
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, repository.ErrModelNotFound) {
		return nil, err
	}

	record, err = s.modelRepository.GetDefault()
	if err != nil {
		return nil, err
	}
	log.Logger.Debug("translate kefu message fallback to default model",
		zap.Int64("modelId", record.ModelID))
	return record, nil
}

// translateTargetLanguage 返回配置的默认目标语言，未配置时用 zh-CN
func translateTargetLanguage() string {
	value := strings.TrimSpace(setting.GetString(configKeyTranslateTargetLanguage, defaultTranslateTargetLanguage))
	if value == "" {
		return defaultTranslateTargetLanguage
	}
	return value
}

// translateUserPrompt 组装翻译的请求内容，附上源语言与目标语言
func translateUserPrompt(text, source, target string) string {
	var builder strings.Builder
	if source != "" {
		builder.WriteString("源语言: " + source + "\n")
	}
	builder.WriteString("目标语言: " + target + "\n待翻译内容:\n")
	builder.WriteString(text)
	return builder.String()
}
