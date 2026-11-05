package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/encrypt"
	"admin/server/pkg/log"
)

// 模型配置的默认值，与数据库字段默认值保持一致
const (
	// defaultModelTemperature 是未配置采样温度时的默认值
	defaultModelTemperature = 0.7
	// defaultModelTopP 是未配置核采样时的默认值
	defaultModelTopP = 1
	// defaultModelMaxTokens 是未配置最大回复 token 时的默认值
	defaultModelMaxTokens = 2048
)

// 连通性测试的参数
const (
	// modelTestMaxTokens 是测试只生成 1 个 token，尽量少消耗额度
	modelTestMaxTokens = 1
	// modelTestTimeout 是连通性测试的超时时间，避免错误的地址长时间挂起
	modelTestTimeout = 20 * time.Second
	// modelTestMessage 是测试时发送的内容
	modelTestMessage = "ping"
)

// 模型配置的业务错误
var (
	// ErrModelNotFound 是模型配置不存在
	ErrModelNotFound = repository.ErrModelNotFound
	// ErrModelNameRequired 是模型名称不能为空
	ErrModelNameRequired = errors.New("模型名称不能为空")
	// ErrModelBaseURLRequired 是接口地址不能为空
	ErrModelBaseURLRequired = errors.New("接口地址不能为空")
	// ErrModelIdentifierRequired 是模型标识不能为空
	ErrModelIdentifierRequired = errors.New("模型标识不能为空")
	// ErrModelApiKeyRequired 是接口密钥不能为空
	ErrModelApiKeyRequired = errors.New("接口密钥不能为空")
	// ErrModelStatusInvalid 是模型状态不正确
	ErrModelStatusInvalid = errors.New("模型状态不正确")
	// ErrModelDefaultInvalid 是默认标记不正确
	ErrModelDefaultInvalid = errors.New("默认标记不正确")
	// ErrModelTranslateInvalid 是翻译模型标记不正确
	ErrModelTranslateInvalid = errors.New("翻译模型标记不正确")
	// ErrModelTestFailed 是模型连通性测试失败
	ErrModelTestFailed = errors.New("模型连接失败，请检查接口地址、密钥与模型标识")
)

// KefuModelService 是模型配置的业务处理
type KefuModelService struct {
	// 模型配置的数据访问
	modelRepository *repository.KefuModelRepository
}

// NewKefuModelService 创建模型配置的业务处理
func NewKefuModelService() *KefuModelService {
	return &KefuModelService{
		modelRepository: repository.NewKefuModelRepository(),
	}
}

// Page 分页查询模型配置，输出前把密钥换成掩码
func (s *KefuModelService) Page(
	param model.KefuModelParam,
) ([]model.KefuModel, int64, error) {
	list, count, err := s.modelRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	fillModelKeyMask(list)
	return list, count, nil
}

// List 查询启用中的模型配置，下拉选择时使用
func (s *KefuModelService) List() ([]model.KefuModel, error) {
	list, err := s.modelRepository.ListEnabled()
	if err != nil {
		return nil, err
	}
	fillModelKeyMask(list)
	return list, nil
}

// Add 新增模型配置，标记为默认时先取消其它模型的默认标记
func (s *KefuModelService) Add(form model.KefuModelForm) error {
	if err := normalizeModelForm(&form, true); err != nil {
		return err
	}
	if form.IsDefault == model.KefuModelIsDefault {
		if err := s.modelRepository.ClearDefault(0); err != nil {
			return err
		}
	}

	encrypted, err := encrypt.Encrypt(form.ApiKey)
	if err != nil {
		return err
	}
	// embedding 密钥留空表示沿用对话模型的密钥
	embeddingKey := ""
	if form.EmbeddingApiKey != "" {
		embeddingKey, err = encrypt.Encrypt(form.EmbeddingApiKey)
		if err != nil {
			return err
		}
	}

	now := model.Now()
	return s.modelRepository.Create(&model.KefuModel{
		Name:             form.Name,
		Provider:         form.Provider,
		BaseURL:          form.BaseURL,
		ApiKey:           encrypted,
		Model:            form.Model,
		Temperature:      form.Temperature,
		TopP:             form.TopP,
		MaxTokens:        form.MaxTokens,
		EmbeddingModel:   form.EmbeddingModel,
		EmbeddingBaseURL: form.EmbeddingBaseURL,
		EmbeddingApiKey:  embeddingKey,
		IsDefault:        form.IsDefault,
		TranslateEnabled: form.TranslateEnabled,
		Status:           model.KefuModelStatusNormal,
		CreateTime:       now,
		UpdateTime:       now,
	})
}

// Update 修改模型配置，接口密钥留空表示不修改
func (s *KefuModelService) Update(form model.KefuModelForm) error {
	record, err := s.modelRepository.GetByID(form.ModelID)
	if err != nil {
		return err
	}
	if err := normalizeModelForm(&form, false); err != nil {
		return err
	}
	if form.IsDefault == model.KefuModelIsDefault {
		if err := s.modelRepository.ClearDefault(form.ModelID); err != nil {
			return err
		}
	}

	record.Name = form.Name
	record.Provider = form.Provider
	record.BaseURL = form.BaseURL
	record.Model = form.Model
	record.Temperature = form.Temperature
	record.TopP = form.TopP
	record.MaxTokens = form.MaxTokens
	record.EmbeddingModel = form.EmbeddingModel
	record.EmbeddingBaseURL = form.EmbeddingBaseURL
	record.IsDefault = form.IsDefault
	record.TranslateEnabled = form.TranslateEnabled
	// 密钥留空表示不修改，避免前端回显密文
	if form.ApiKey != "" {
		encrypted, err := encrypt.Encrypt(form.ApiKey)
		if err != nil {
			return err
		}
		record.ApiKey = encrypted
	}
	// embedding 密钥单独提交时覆盖，提交空值表示继续沿用对话模型的密钥
	if form.EmbeddingApiKey != "" {
		encrypted, err := encrypt.Encrypt(form.EmbeddingApiKey)
		if err != nil {
			return err
		}
		record.EmbeddingApiKey = encrypted
	}
	record.UpdateTime = model.Now()
	return s.modelRepository.Update(record)
}

// UpdateStatus 修改模型配置的状态，停用默认模型时同时取消默认标记，
// 避免 AI 自动回复继续使用已停用的模型
func (s *KefuModelService) UpdateStatus(form model.KefuModelStatusForm) error {
	if _, err := s.modelRepository.GetByID(form.ModelID); err != nil {
		return err
	}
	if form.Status != model.KefuModelStatusNormal &&
		form.Status != model.KefuModelStatusDisabled {
		return ErrModelStatusInvalid
	}

	fields := map[string]any{
		"status":      form.Status,
		"update_time": model.Now(),
	}
	if form.Status == model.KefuModelStatusDisabled {
		fields["is_default"] = model.KefuModelNotDefault
	}
	return s.modelRepository.UpdateFields(form.ModelID, fields)
}

// Remove 删除模型配置
func (s *KefuModelService) Remove(modelID int64) error {
	if _, err := s.modelRepository.GetByID(modelID); err != nil {
		return err
	}
	return s.modelRepository.Remove(modelID)
}

// Test 用已保存的配置做一次 1 个 token 的调用，验证地址、密钥与模型标识；
// 配置了 embedding 模型时顺带验证向量化接口，避免分开配置后到入库才发现向量侧不通
func (s *KefuModelService) Test(ctx context.Context, modelID int64) error {
	record, err := s.modelRepository.GetByID(modelID)
	if err != nil {
		return err
	}
	options, err := aiclient.FromModel(record)
	if err != nil {
		return err
	}
	client, err := aiclient.New(options)
	if err != nil {
		return err
	}

	testCtx, cancel := context.WithTimeout(ctx, modelTestTimeout)
	defer cancel()

	req := client.Request([]aiclient.Message{{
		Role:    aiclient.RoleUser,
		Content: modelTestMessage,
	}})
	req.MaxTokens = modelTestMaxTokens
	if _, err := client.ChatOnce(testCtx, req); err != nil {
		log.Logger.Warn("test kefu model failed",
			zap.Int64("modelId", modelID), zap.Error(err))
		return fmt.Errorf("%w: %s", ErrModelTestFailed, err)
	}

	// embedding 未配置时不启用知识库检索，也不做向量化测试
	if strings.TrimSpace(record.EmbeddingModel) == "" {
		return nil
	}
	if _, err := client.Embeddings(testCtx, []string{modelTestMessage}); err != nil {
		log.Logger.Warn("test kefu embedding model failed",
			zap.Int64("modelId", modelID), zap.Error(err))
		return fmt.Errorf("%w: embedding 模型调用失败: %s", ErrModelTestFailed, err)
	}
	return nil
}

// fillModelKeyMask 把模型密钥换成掩码并清空密文，避免接口输出密钥
func fillModelKeyMask(list []model.KefuModel) {
	for i := range list {
		plain, err := encrypt.Decrypt(list[i].ApiKey)
		if err != nil {
			log.Logger.Warn("decrypt kefu model api key failed",
				zap.Int64("modelId", list[i].ModelID), zap.Error(err))
		}
		list[i].ApiKey = ""
		list[i].ApiKeyMask = encrypt.Mask(plain)

		// embedding 密钥未单独配置时为空，前端据此提示"沿用对话密钥"
		embeddingPlain, err := encrypt.Decrypt(list[i].EmbeddingApiKey)
		if err != nil {
			log.Logger.Warn("decrypt kefu model embedding api key failed",
				zap.Int64("modelId", list[i].ModelID), zap.Error(err))
		}
		list[i].EmbeddingApiKey = ""
		list[i].EmbeddingApiKeyMask = encrypt.Mask(embeddingPlain)
	}
}

// normalizeModelForm 校验并归一化模型配置的提交数据，
// needApiKey 为 true 时接口密钥必填
func normalizeModelForm(form *model.KefuModelForm, needApiKey bool) error {
	form.Name = strings.TrimSpace(form.Name)
	form.Provider = strings.TrimSpace(form.Provider)
	form.BaseURL = strings.TrimSpace(form.BaseURL)
	form.ApiKey = strings.TrimSpace(form.ApiKey)
	form.Model = strings.TrimSpace(form.Model)
	form.EmbeddingModel = strings.TrimSpace(form.EmbeddingModel)
	form.EmbeddingBaseURL = strings.TrimSpace(form.EmbeddingBaseURL)
	form.EmbeddingApiKey = strings.TrimSpace(form.EmbeddingApiKey)

	if form.Name == "" {
		return ErrModelNameRequired
	}
	if form.BaseURL == "" {
		return ErrModelBaseURLRequired
	}
	if form.Model == "" {
		return ErrModelIdentifierRequired
	}
	if needApiKey && form.ApiKey == "" {
		return ErrModelApiKeyRequired
	}
	if form.Provider == "" {
		form.Provider = model.KefuModelProviderOpenAICompatible
	}
	if form.IsDefault != model.KefuModelNotDefault &&
		form.IsDefault != model.KefuModelIsDefault {
		return ErrModelDefaultInvalid
	}
	if form.TranslateEnabled != model.KefuModelNotTranslate &&
		form.TranslateEnabled != model.KefuModelIsTranslate {
		return ErrModelTranslateInvalid
	}
	if form.Temperature < 0 || form.Temperature > 2 {
		form.Temperature = defaultModelTemperature
	}
	if form.TopP < 0 || form.TopP > 1 {
		form.TopP = defaultModelTopP
	}
	if form.MaxTokens <= 0 {
		form.MaxTokens = defaultModelMaxTokens
	}
	return nil
}
