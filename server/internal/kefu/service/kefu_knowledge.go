package service

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/rag"
	"admin/server/internal/kefu/repository"
	"admin/server/internal/kefu/vector"
	systemservice "admin/server/internal/system/service"
	"admin/server/model"
	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"
	"admin/server/pkg/setting"
)

// 知识库检索在配置中心的配置键
const (
	// configKeyRagTopK 是召回条数
	configKeyRagTopK = "kefu.rag.top_k"
	// configKeyRagThreshold 是召回相似度阈值
	configKeyRagThreshold = "kefu.rag.score_threshold"
	// configKeyRagEmbedBatchSize 是一次向量化的分块数量
	configKeyRagEmbedBatchSize = "kefu.rag.embed_batch_size"
)

// 知识库检索的默认参数，与迁移种子的默认值保持一致
const (
	defaultRagTopK      = 5
	defaultRagThreshold = 0.35
	// defaultEmbedBatchSize 取 10，兼容只接受 10 条以内的 embedding 接口（如百炼 text-embedding-v3）
	defaultEmbedBatchSize = 10
)

// 知识库向量化任务
const (
	// embedQueueKey 是向量化任务的 redis 队列，进程内 worker 串行消费
	embedQueueKey = "kefu:task:embed"
	// embedQueuePollTimeout 是队列空闲时的阻塞时长
	embedQueuePollTimeout = 5 * time.Second
	// embedTaskTimeout 是单篇文档入库的总超时
	embedTaskTimeout = 10 * time.Minute
	// embedErrorLimit 是失败原因的最大长度，避免异常堆栈撑爆字段
	embedErrorLimit = 500
	// uploadDir 是系统模块的文件上传根目录，知识库解析时从磁盘读取原文
	uploadDir = "uploads"
)

// errEmbedQueueEmpty 是队列当前没有待处理任务
var errEmbedQueueEmpty = errors.New("知识库任务队列为空")

// 常见的业务错误
var (
	// ErrKnowledgeNotFound 是知识库文档不存在
	ErrKnowledgeNotFound = repository.ErrKnowledgeNotFound
	// ErrKnowledgeFileNotFound 是知识库关联的上传文件不存在
	ErrKnowledgeFileNotFound = repository.ErrUploadFileNotFound
	// ErrKnowledgeCategoryRequired 是未选择文档库分类
	ErrKnowledgeCategoryRequired = errors.New("请选择文档库分类")
	// ErrKnowledgeCategoryInvalid 是文档库分类不存在或已停用
	ErrKnowledgeCategoryInvalid = errors.New("文档库分类不存在或已停用")
	// ErrKnowledgeTitleRequired 是文档标题不能为空
	ErrKnowledgeTitleRequired = errors.New("文档标题不能为空")
	// ErrKnowledgeFileRequired 是既没有上传文件也没有录入内容
	ErrKnowledgeFileRequired = errors.New("请选择要入库的文档")
	// ErrKnowledgeSourceInvalid 是文档来源取值不正确
	ErrKnowledgeSourceInvalid = errors.New("文档来源取值不正确")
	// ErrKnowledgeUnsupportedExt 是文档格式不支持
	ErrKnowledgeUnsupportedExt = rag.ErrUnsupportedExt
	// ErrKnowledgeContentEmpty 是文档没有可入库的文本
	ErrKnowledgeContentEmpty = errors.New("文档没有可入库的文本内容，请检查文件是否为空或为扫描件")
	// ErrKnowledgeQueryRequired 是召回测试的检索内容为空
	ErrKnowledgeQueryRequired = errors.New("请输入要检索的内容")
	// ErrRagUnavailable 是知识库检索不可用，按未启用 RAG 处理
	ErrRagUnavailable = errors.New("知识库检索未启用，请先配置向量库与 embedding 模型")
)

// KefuKnowledgeService 是知识库的业务处理：
// 上传入库 → 解析分块 → 向量化写向量库 → 检索召回
type KefuKnowledgeService struct {
	// 知识库文档的数据访问
	knowledgeRepository *repository.KefuKnowledgeRepository
	// 知识库分块的数据访问
	chunkRepository *repository.KefuKnowledgeChunkRepository
	// 上传文件记录的数据访问
	fileRepository *repository.KefuFileRepository
	// 模型配置的数据访问，取默认模型的 embedding 配置
	modelRepository *repository.KefuModelRepository
	// 文档库分类的数据访问，校验分类与回填分类名称
	categoryRepository *repository.KefuKnowledgeCategoryRepository
}

// NewKefuKnowledgeService 创建知识库的业务处理
func NewKefuKnowledgeService() *KefuKnowledgeService {
	return &KefuKnowledgeService{
		knowledgeRepository: repository.NewKefuKnowledgeRepository(),
		chunkRepository:     repository.NewKefuKnowledgeChunkRepository(),
		fileRepository:      repository.NewKefuFileRepository(),
		modelRepository:     repository.NewKefuModelRepository(),
		categoryRepository:  repository.NewKefuKnowledgeCategoryRepository(),
	}
}

// Page 分页查询知识库文档，并回填文档库分类名称
func (s *KefuKnowledgeService) Page(
	param model.KefuKnowledgeParam,
) ([]model.KefuKnowledge, int64, error) {
	list, count, err := s.knowledgeRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	s.fillCategoryNames(list)
	return list, count, nil
}

// Add 新增知识库文档并提交向量化任务；
// 关联文件走 /file/upload，直接录入的文本先落成 txt 上传文件，两者共用同一条流水线
func (s *KefuKnowledgeService) Add(
	form model.KefuKnowledgeForm, userID int64,
) (*model.KefuKnowledge, error) {
	if err := s.checkCategory(form.CategoryID); err != nil {
		return nil, err
	}

	source := strings.TrimSpace(form.Source)
	if source == "" {
		source = model.KefuKnowledgeSourceUpload
	}
	if source != model.KefuKnowledgeSourceUpload {
		return nil, ErrKnowledgeSourceInvalid
	}

	now := model.Now()
	record := &model.KefuKnowledge{
		CategoryID:   form.CategoryID,
		Title:        strings.TrimSpace(form.Title),
		FileID:       form.FileID,
		Source:       source,
		Status:       model.KefuKnowledgeStatusPending,
		CreateUserID: userID,
		CreateTime:   now,
		UpdateTime:   now,
	}

	switch {
	case form.FileID > 0:
		file, err := s.fileRepository.GetByID(form.FileID)
		if err != nil {
			return nil, err
		}
		record.FileName = file.Name
		record.FileSize = file.Length
		record.FileExt = rag.NormalizeExt(filepath.Ext(file.Name))
		if record.Title == "" {
			record.Title = file.Name
		}
	case strings.TrimSpace(form.Content) != "":
		file, err := s.saveTextFile(record.Title, form.Content, userID)
		if err != nil {
			return nil, err
		}
		record.FileID = file.ID
		record.FileName = file.Name
		record.FileSize = file.Length
		record.FileExt = rag.ExtText
	default:
		return nil, ErrKnowledgeFileRequired
	}

	if record.Title == "" {
		return nil, ErrKnowledgeTitleRequired
	}
	if !rag.Supported(record.FileExt) {
		return nil, ErrKnowledgeUnsupportedExt
	}

	if err := s.knowledgeRepository.Create(record); err != nil {
		return nil, err
	}
	s.schedule(record.DocID)
	return record, nil
}

// Remove 删除知识库文档：先删向量库里的分块，再删库里的分块与文档
func (s *KefuKnowledgeService) Remove(docID int64) error {
	record, err := s.knowledgeRepository.GetByID(docID)
	if err != nil {
		return err
	}
	if err := s.removeVectors(context.Background(), record.DocID); err != nil {
		return err
	}
	if err := s.chunkRepository.RemoveByDoc(record.DocID); err != nil {
		return err
	}
	return s.knowledgeRepository.Remove(record.DocID)
}

// Reindex 重建文档索引：把状态退回待处理并重新提交向量化任务，
// 旧的向量与分块在流水线开始时清理
func (s *KefuKnowledgeService) Reindex(docID int64) (*model.KefuKnowledge, error) {
	record, err := s.knowledgeRepository.GetByID(docID)
	if err != nil {
		return nil, err
	}
	if err := s.knowledgeRepository.UpdateFields(record.DocID, map[string]any{
		"status":      model.KefuKnowledgeStatusPending,
		"error":       "",
		"chunk_count": 0,
	}); err != nil {
		return nil, err
	}

	record.Status = model.KefuKnowledgeStatusPending
	record.Error = ""
	record.ChunkCount = 0
	s.schedule(record.DocID)
	return record, nil
}

// Update 修改知识库文档的标题与文档库分类；
// 不触发重新向量化，改了分类后需要重建索引才能让 payload 里的 category_id 生效
func (s *KefuKnowledgeService) Update(form model.KefuKnowledgeForm) error {
	record, err := s.knowledgeRepository.GetByID(form.DocID)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(form.Title)
	if title == "" {
		return ErrKnowledgeTitleRequired
	}
	if err := s.checkCategory(form.CategoryID); err != nil {
		return err
	}

	if err := s.knowledgeRepository.UpdateFields(record.DocID, map[string]any{
		"title":       title,
		"category_id": form.CategoryID,
	}); err != nil {
		return err
	}
	record.Title = title
	record.CategoryID = form.CategoryID
	return nil
}

// Search 按文档库分类召回知识库，召回测试与 AI 编排共用
func (s *KefuKnowledgeService) Search(
	ctx context.Context, form model.KefuKnowledgeSearchForm,
) ([]model.KefuKnowledgeHit, error) {
	categoryIDs := uniqueIDs(form.CategoryIDs)
	if len(categoryIDs) == 0 {
		return nil, ErrKnowledgeCategoryRequired
	}
	query := strings.TrimSpace(form.Query)
	if query == "" {
		return nil, ErrKnowledgeQueryRequired
	}

	embed, err := s.embedFunc()
	if err != nil {
		return nil, err
	}
	hits, err := rag.Retrieve(ctx, embed, rag.Query{
		CategoryIDs: categoryIDs,
		Text:        query,
		TopK:        s.topK(form.TopK),
		Threshold:   s.threshold(form.Threshold),
	})
	if err != nil {
		if errors.Is(err, vector.ErrNotInitialized) {
			return nil, ErrRagUnavailable
		}
		return nil, err
	}
	return s.loadHits(hits), nil
}

// StartEmbedTask 启动知识库向量化任务，按 redis 队列串行消费
func (s *KefuKnowledgeService) StartEmbedTask() {
	go func() {
		for {
			docID, err := s.popEmbedTask()
			if err != nil {
				if errors.Is(err, errEmbedQueueEmpty) {
					continue
				}
				log.Logger.Warn("pop kefu knowledge task failed", zap.Error(err))
				// redis 异常时避免空转
				time.Sleep(time.Second)
				continue
			}
			s.runEmbedTask(docID)
		}
	}()
	log.Kernel.Info("start kefu knowledge embed task")
}

// schedule 提交向量化任务；队列不可用时退化为进程内异步执行，避免文档一直停在待处理
func (s *KefuKnowledgeService) schedule(docID int64) {
	if docID <= 0 {
		return
	}
	if rdb.Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := rdb.Client.LPush(ctx, embedQueueKey, docID).Err()
		cancel()
		if err == nil {
			return
		}
		log.Logger.Warn("push kefu knowledge task failed, run inline",
			zap.Int64("docId", docID), zap.Error(err))
	}
	go s.runEmbedTask(docID)
}

// popEmbedTask 从队列取出一个待向量化的文档id
func (s *KefuKnowledgeService) popEmbedTask() (int64, error) {
	if rdb.Client == nil {
		return 0, errors.New("redis 未初始化，无法消费知识库任务")
	}
	values, err := rdb.Client.BRPop(
		context.Background(), embedQueuePollTimeout, embedQueueKey).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, errEmbedQueueEmpty
		}
		return 0, err
	}
	if len(values) < 2 {
		return 0, errEmbedQueueEmpty
	}
	docID, err := strconv.ParseInt(strings.TrimSpace(values[1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("知识库任务 id 不正确: %w", err)
	}
	return docID, nil
}

// runEmbedTask 执行一次完整的入库流程，失败时把原因写回文档便于重试
func (s *KefuKnowledgeService) runEmbedTask(docID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), embedTaskTimeout)
	defer cancel()

	if err := s.embed(ctx, docID); err != nil {
		log.Logger.Error("embed kefu knowledge failed",
			zap.Int64("docId", docID), zap.Error(err))
		if updateErr := s.knowledgeRepository.UpdateFields(docID, map[string]any{
			"status": model.KefuKnowledgeStatusFailed,
			"error":  truncateText(err.Error(), embedErrorLimit),
		}); updateErr != nil {
			log.Logger.Warn("mark kefu knowledge failed",
				zap.Int64("docId", docID), zap.Error(updateErr))
		}
		// 失败时清掉本次可能已写入的分块与向量，避免半成品被召回
		s.cleanupAfterFailure(ctx, docID)
	}
}

// cleanupAfterFailure 失败后清理文档的分块与向量，清理失败只记日志，可人工重建索引
func (s *KefuKnowledgeService) cleanupAfterFailure(ctx context.Context, docID int64) {
	if err := s.removeVectors(ctx, docID); err != nil {
		log.Logger.Warn("clean kefu knowledge vectors failed",
			zap.Int64("docId", docID), zap.Error(err))
	}
	if err := s.chunkRepository.RemoveByDoc(docID); err != nil {
		log.Logger.Warn("clean kefu knowledge chunks failed",
			zap.Int64("docId", docID), zap.Error(err))
	}
}

// embed 解析文档 → 分块落库 → 向量化 → 写向量库 → 标记就绪
func (s *KefuKnowledgeService) embed(ctx context.Context, docID int64) error {
	record, err := s.knowledgeRepository.GetByID(docID)
	if err != nil {
		// 文档在排队期间被删除，直接跳过
		if errors.Is(err, repository.ErrKnowledgeNotFound) {
			return nil
		}
		return err
	}
	if err := s.markStatus(record, model.KefuKnowledgeStatusParsing); err != nil {
		return err
	}

	text, err := s.loadText(ctx, record)
	if err != nil {
		return err
	}
	chunks, err := rag.Split(text, record.FileExt)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return ErrKnowledgeContentEmpty
	}

	// 重建索引时先清掉旧的分块与向量；没有分块说明是首次入库，跳过向量删除
	count, err := s.chunkRepository.CountByDoc(record.DocID)
	if err != nil {
		return err
	}
	if count > 0 {
		if err := s.removeVectors(ctx, record.DocID); err != nil {
			return err
		}
		if err := s.chunkRepository.RemoveByDoc(record.DocID); err != nil {
			return err
		}
	}

	chatClient, err := s.embeddingClient()
	if err != nil {
		return err
	}
	if err := s.markStatus(record, model.KefuKnowledgeStatusEmbedding); err != nil {
		return err
	}

	now := model.Now()
	list := make([]model.KefuKnowledgeChunk, 0, len(chunks))
	for _, chunk := range chunks {
		list = append(list, model.KefuKnowledgeChunk{
			DocID:      record.DocID,
			Seq:        chunk.Seq,
			Content:    chunk.Content,
			Tokens:     chunk.Tokens,
			VectorID:   generateVectorID(),
			CreateTime: now,
			UpdateTime: now,
		})
	}
	if err := s.chunkRepository.CreateBatch(list); err != nil {
		return err
	}

	batchSize := setting.GetInt(configKeyRagEmbedBatchSize, defaultEmbedBatchSize)
	if batchSize < 1 {
		batchSize = defaultEmbedBatchSize
	}
	for start := 0; start < len(list); start += batchSize {
		end := start + batchSize
		if end > len(list) {
			end = len(list)
		}
		if err := s.upsertBatch(ctx, record, list[start:end], chatClient); err != nil {
			return err
		}
	}

	return s.knowledgeRepository.UpdateFields(record.DocID, map[string]any{
		"status":      model.KefuKnowledgeStatusReady,
		"error":       "",
		"chunk_count": len(list),
	})
}

// upsertBatch 向量化一批分块并写入向量库
func (s *KefuKnowledgeService) upsertBatch(
	ctx context.Context, record *model.KefuKnowledge,
	list []model.KefuKnowledgeChunk, chatClient *aiclient.Client,
) error {
	texts := make([]string, 0, len(list))
	for _, chunk := range list {
		texts = append(texts, chunk.Content)
	}

	vectors, err := chatClient.Embeddings(ctx, texts)
	if err != nil {
		return err
	}
	if len(vectors) != len(list) {
		return fmt.Errorf("向量化结果数量不正确: 期望 %d 条, 实际 %d 条", len(list), len(vectors))
	}

	points := make([]vector.Point, 0, len(list))
	for i, chunk := range list {
		if len(vectors[i]) == 0 {
			return fmt.Errorf("分块 %d 向量化结果为空", chunk.Seq)
		}
		points = append(points, vector.Point{
			ID:      chunk.VectorID,
			Vector:  vectors[i],
			Payload: rag.NewKnowledgePayload(record.DocID, record.CategoryID, record.Title, chunk.Seq),
		})
	}
	return vector.Default.Upsert(ctx, rag.KnowledgeCollection, points)
}

// loadText 读取文档原文，上传的文件从磁盘解析，直接录入的文本已经是 txt 文件
func (s *KefuKnowledgeService) loadText(
	ctx context.Context, record *model.KefuKnowledge,
) (string, error) {
	file, err := s.fileRepository.GetByID(record.FileID)
	if err != nil {
		return "", err
	}
	path := filepath.Join(uploadDir, filepath.FromSlash(file.Path))
	return rag.Load(ctx, path, record.FileExt)
}

// markStatus 更新文档处理状态，同时清掉上一次的失败原因
func (s *KefuKnowledgeService) markStatus(record *model.KefuKnowledge, status int) error {
	if err := s.knowledgeRepository.UpdateFields(record.DocID, map[string]any{
		"status": status,
		"error":  "",
	}); err != nil {
		return err
	}
	record.Status = status
	record.Error = ""
	return nil
}

// removeVectors 按 doc_id 删除向量库里的分块，向量库未配置时直接跳过
func (s *KefuKnowledgeService) removeVectors(ctx context.Context, docID int64) error {
	if vector.Default == nil {
		return nil
	}
	filter := map[string]string{rag.PayloadDocID: strconv.FormatInt(docID, 10)}
	return vector.Default.DeleteByFilter(ctx, rag.KnowledgeCollection, filter)
}

// loadHits 按向量点 id 回查分块内容，取不到内容的命中直接丢弃
func (s *KefuKnowledgeService) loadHits(hits []rag.Hit) []model.KefuKnowledgeHit {
	list := make([]model.KefuKnowledgeHit, 0, len(hits))
	if len(hits) == 0 {
		return list
	}

	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		if hit.VectorID != "" {
			ids = append(ids, hit.VectorID)
		}
	}
	chunks, err := s.chunkRepository.ListByVectorIDs(ids)
	if err != nil {
		log.Logger.Warn("load kefu knowledge chunks failed", zap.Error(err))
		return list
	}

	contents := make(map[string]string, len(chunks))
	for _, chunk := range chunks {
		contents[chunk.VectorID] = chunk.Content
	}
	for _, hit := range hits {
		content, ok := contents[hit.VectorID]
		if !ok {
			continue
		}
		list = append(list, model.KefuKnowledgeHit{
			DocID:    hit.DocID,
			Title:    hit.Title,
			ChunkSeq: hit.ChunkSeq,
			Content:  content,
			Score:    hit.Score,
		})
	}
	return list
}

// embedFunc 用默认模型的 embedding 配置构造向量化函数
func (s *KefuKnowledgeService) embedFunc() (rag.EmbedFunc, error) {
	chatClient, err := s.embeddingClient()
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, texts []string) ([][]float32, error) {
		return chatClient.Embeddings(ctx, texts)
	}, nil
}

// embeddingClient 按默认模型配置创建向量化客户端，向量库未配置时视为未启用 RAG
func (s *KefuKnowledgeService) embeddingClient() (*aiclient.Client, error) {
	if vector.Default == nil {
		return nil, ErrRagUnavailable
	}

	record, err := s.modelRepository.GetDefault()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRagUnavailable, err)
	}
	if strings.TrimSpace(record.EmbeddingModel) == "" {
		return nil, fmt.Errorf("%w: 模型配置里没有填 embedding 模型标识", ErrRagUnavailable)
	}
	options, err := aiclient.FromModel(record)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRagUnavailable, err)
	}
	return aiclient.New(options)
}

// checkCategory 校验文档库分类存在且启用，避免写入无法检索的脏数据
func (s *KefuKnowledgeService) checkCategory(categoryID int64) error {
	if categoryID <= 0 {
		return ErrKnowledgeCategoryRequired
	}
	record, err := s.categoryRepository.GetByID(categoryID)
	if err != nil {
		if errors.Is(err, repository.ErrKnowledgeCategoryNotFound) {
			return ErrKnowledgeCategoryInvalid
		}
		return err
	}
	if record.Status != model.KefuKnowledgeCategoryStatusNormal {
		return ErrKnowledgeCategoryInvalid
	}
	return nil
}

// fillCategoryNames 批量回填文档库分类名称，查询不到时留空
func (s *KefuKnowledgeService) fillCategoryNames(list []model.KefuKnowledge) {
	if len(list) == 0 {
		return
	}

	categoryIDs := make([]int64, 0, len(list))
	for _, item := range list {
		categoryIDs = append(categoryIDs, item.CategoryID)
	}
	categories, err := s.categoryRepository.ListByIDs(uniqueIDs(categoryIDs))
	if err != nil {
		log.Logger.Warn("list kefu knowledge categories failed", zap.Error(err))
		return
	}
	names := make(map[int64]string, len(categories))
	for _, category := range categories {
		names[category.CategoryID] = category.Name
	}
	for i := range list {
		list[i].CategoryName = names[list[i].CategoryID]
	}
}

// saveTextFile 把直接录入的文本存成 txt 上传文件，复用系统模块的文件服务，
// 让文本与上传文档走同一条解析、分块、向量化流水线
func (s *KefuKnowledgeService) saveTextFile(
	title, content string, userID int64,
) (*model.SysFile, error) {
	name := strings.TrimSpace(title)
	if name == "" {
		name = "知识库文本"
	}
	if !strings.HasSuffix(strings.ToLower(name), "."+rag.ExtText) {
		name += "." + rag.ExtText
	}

	text := content
	fileService := systemservice.NewSysFileService()
	record, err := fileService.Save(name, strings.NewReader(text),
		int64(len(text)), "text/plain; charset=utf-8", userID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// topK 返回本次召回的条数，未指定时取配置中心的值
func (s *KefuKnowledgeService) topK(value int) int {
	if value > 0 {
		return value
	}
	topK := setting.GetInt(configKeyRagTopK, defaultRagTopK)
	if topK <= 0 {
		return defaultRagTopK
	}
	return topK
}

// threshold 返回本次召回的相似度阈值，未指定时取配置中心的值
func (s *KefuKnowledgeService) threshold(value float64) float64 {
	if value > 0 {
		return value
	}
	threshold := setting.GetFloat(configKeyRagThreshold, defaultRagThreshold)
	if threshold <= 0 {
		return defaultRagThreshold
	}
	return threshold
}

// knowledgeText 把召回命中拼成提示词里的知识库内容，没有命中时返回空串
func knowledgeText(hits []model.KefuKnowledgeHit) string {
	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("【%s】%s", hit.Title, content))
	}
	return strings.Join(parts, "\n\n")
}

// truncateText 按字符数截断文本，用于失败原因等有长度上限的字段
func truncateText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

// generateVectorID 生成向量点的 uuid，向量库的 uuid 点位要求标准格式
func generateVectorID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// 随机源不可用时用时间戳兜底，仍然保证格式合法
		now := uint64(time.Now().UnixNano())
		binary.BigEndian.PutUint64(buf[0:8], now)
		binary.BigEndian.PutUint64(buf[8:16], now>>1)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // 版本4
	buf[8] = (buf[8] & 0x3f) | 0x80 // RFC4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
