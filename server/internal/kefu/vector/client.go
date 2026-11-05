// Package vector 是 Qdrant 向量库的封装，只做集合与向量的读写，不含客服业务；
// 集合名由调用方给基础名，实际名称统一加配置里的前缀，如 kefu_knowledge
package vector

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/qdrant/go-client/qdrant"

	"admin/server/pkg/config"
	"admin/server/pkg/log"
)

// defaultPort 是 Qdrant gRPC 的默认端口
const defaultPort = 6334

// defaultPrefix 是集合名前缀未配置时使用的值
const defaultPrefix = "kefu"

// defaultVectorSize 是向量维度未配置时使用的值
const defaultVectorSize = 1536

// requestTimeout 是单次向量库操作的超时
const requestTimeout = 30 * time.Second

// ErrNotInitialized 是向量库客户端未初始化
var ErrNotInitialized = errors.New("向量库未初始化")

// ErrUnavailable 是向量库不可达，业务侧按"未启用 RAG"处理
var ErrUnavailable = errors.New("向量库不可达")

// Point 是一个待写入的向量点
type Point struct {
	// point id, 为空时由调用方保证唯一, 建议用 uuid
	ID string
	// 向量
	Vector []float32
	// 附加信息, 检索时按它过滤
	Payload map[string]any
}

// SearchResult 是一条检索命中结果
type SearchResult struct {
	// point id
	ID string
	// 相似度得分
	Score float32
	// 附加信息
	Payload map[string]any
}

// Filter 是检索的 payload 过滤条件；
// Must 为全部命中，Any 为任一命中
// Filter 是检索的 payload 过滤条件，
// Must 为全部命中，Any 为任一命中
type Filter struct {
	// 全部命中的条件
	Must map[string]string
	// 任一命中的条件，如多个文档库分类
	Any map[string][]string
}

// Client 是 Qdrant 的客户端
type Client struct {
	// 底层客户端
	client *qdrant.Client
	// 集合名前缀
	prefix string
	// 向量维度
	vectorSize int
}

// Default 是全局的向量库客户端，Init 之后可用；不可达时为 nil
var Default *Client

// Init 用配置文件里的向量库配置初始化全局客户端；
// 向量库不可达时只记 Warn 不阻塞服务启动，此时 Default 为 nil
func Init(cfg config.Qdrant) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		log.Logger.Warn("未配置 qdrant.url，知识库检索功能不可用")
		return
	}

	host, port, useTLS, err := parseURL(rawURL)
	if err != nil {
		log.Logger.Warn("qdrant.url 不合法，知识库检索功能不可用",
			zap.String("url", rawURL), zap.Error(err))
		return
	}

	prefix := strings.TrimSpace(cfg.CollectionPrefix)
	if prefix == "" {
		prefix = defaultPrefix
	}
	vectorSize := cfg.VectorSize
	if vectorSize <= 0 {
		vectorSize = defaultVectorSize
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   host,
		Port:   port,
		APIKey: cfg.ApiKey,
		UseTLS: useTLS,
		// 跳过版本兼容检测，避免向量库不可达时启动被版本探测卡住
		SkipCompatibilityCheck: true,
		PoolSize:               1,
	})
	if err != nil {
		log.Logger.Warn("连接 qdrant 失败，知识库检索功能不可用",
			zap.String("url", rawURL), zap.Error(err))
		return
	}

	// 探活失败只记 Warn，等向量库恢复后重建客户端即可
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := client.ListCollections(ctx); err != nil {
		log.Logger.Warn("qdrant 探活失败，知识库检索功能暂不可用",
			zap.String("url", rawURL), zap.Error(err))
	}

	Default = &Client{client: client, prefix: prefix, vectorSize: vectorSize}
	log.Kernel.Info("init qdrant client",
		zap.String("url", rawURL), zap.String("prefix", prefix), zap.Int("vectorSize", vectorSize))
}

// parseURL 解析向量库地址，支持 host:port 与 http(s)://host:port 两种写法
func parseURL(rawURL string) (host string, port int, useTLS bool, err error) {
	value := rawURL
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", 0, false, err
	}
	host = parsed.Hostname()
	if host == "" {
		return "", 0, false, errors.New("地址缺少主机名")
	}

	port = defaultPort
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return "", 0, false, fmt.Errorf("端口 %q 不正确: %w", parsed.Port(), err)
		}
	}
	return host, port, parsed.Scheme == "https", nil
}

// CollectionName 返回加过前缀的集合名，如 knowledge 对应 kefu_knowledge
func (c *Client) CollectionName(name string) string {
	if c == nil {
		return name
	}
	return c.prefix + "_" + name
}

// EnsureCollection 集合不存在时按向量维度创建，size 传 0 时用配置里的维度
func (c *Client) EnsureCollection(ctx context.Context, name string, size int) error {
	if c == nil {
		return ErrNotInitialized
	}
	if size <= 0 {
		size = c.vectorSize
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	collection := c.CollectionName(name)
	exists, err := c.client.CollectionExists(ctx, collection)
	if err != nil {
		return fmt.Errorf("检查向量集合失败: %w", err)
	}
	if exists {
		return nil
	}

	err = c.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(size),
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("创建向量集合失败: %w", err)
	}
	log.Logger.Debug("vector collection created",
		zap.String("collection", collection), zap.Int("size", size))
	return nil
}

// Upsert 批量写入或覆盖向量点，集合不存在时先按配置的维度创建
func (c *Client) Upsert(ctx context.Context, name string, points []Point) error {
	if c == nil {
		return ErrNotInitialized
	}
	if len(points) == 0 {
		return nil
	}
	if err := c.EnsureCollection(ctx, name, 0); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	list := make([]*qdrant.PointStruct, 0, len(points))
	for _, point := range points {
		if strings.TrimSpace(point.ID) == "" {
			return errors.New("向量点的 id 不能为空")
		}
		list = append(list, &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(point.ID),
			Vectors: qdrant.NewVectors(point.Vector...),
			Payload: qdrant.NewValueMap(point.Payload),
		})
	}

	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: c.CollectionName(name),
		Points:         list,
	})
	if err != nil {
		return fmt.Errorf("写入向量失败: %w", err)
	}
	return nil
}

// Search 按向量检索，filter 为 payload 的精确匹配条件，threshold 大于 0 时生效
func (c *Client) Search(
	ctx context.Context,
	name string,
	vec []float32,
	filter map[string]string,
	topK int,
	threshold float64,
) ([]SearchResult, error) {
	return c.SearchWithFilter(ctx, name, vec, &Filter{Must: filter}, topK, threshold)
}

// SearchWithFilter 按向量检索，filter 支持全部命中与任一命中两种条件，
// threshold 大于 0 时生效
func (c *Client) SearchWithFilter(
	ctx context.Context,
	name string,
	vec []float32,
	filter *Filter,
	topK int,
	threshold float64,
) ([]SearchResult, error) {
	if c == nil {
		return nil, ErrNotInitialized
	}
	if len(vec) == 0 {
		return nil, errors.New("检索向量不能为空")
	}
	if topK <= 0 {
		topK = 5
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	limit := uint64(topK)
	request := &qdrant.QueryPoints{
		CollectionName: c.CollectionName(name),
		Query:          qdrant.NewQuery(vec...),
		Limit:          &limit,
		Filter:         buildFilterConditions(filter),
		WithPayload:    qdrant.NewWithPayloadEnable(true),
	}
	if threshold > 0 {
		score := float32(threshold)
		request.ScoreThreshold = &score
	}

	points, err := c.client.Query(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("检索向量失败: %w", err)
	}

	list := make([]SearchResult, 0, len(points))
	for _, point := range points {
		list = append(list, SearchResult{
			ID:      pointID(point.GetId()),
			Score:   point.GetScore(),
			Payload: qdrant.ValueMapToMap(point.GetPayload()),
		})
	}
	return list, nil
}

// DeleteByFilter 按 payload 条件删除向量点，为空条件时不执行，避免误删整个集合；
// 集合还不存在说明没有写入过向量，直接当作删除完成
func (c *Client) DeleteByFilter(ctx context.Context, name string, filter map[string]string) error {
	if c == nil {
		return ErrNotInitialized
	}
	conditions := buildFilter(filter)
	if conditions == nil {
		return errors.New("删除向量必须指定过滤条件")
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	collection := c.CollectionName(name)
	exists, err := c.client.CollectionExists(ctx, collection)
	if err != nil {
		return fmt.Errorf("检查向量集合失败: %w", err)
	}
	if !exists {
		return nil
	}

	_, err = c.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points:         qdrant.NewPointsSelectorFilter(conditions),
	})
	if err != nil {
		return fmt.Errorf("删除向量失败: %w", err)
	}
	return nil
}

// buildFilter 把精确匹配条件转换为向量库的过滤条件，条件为空时返回 nil
func buildFilter(filter map[string]string) *qdrant.Filter {
	return buildFilterConditions(&Filter{Must: filter})
}

// buildFilterConditions 把检索过滤条件转换为向量库的过滤条件，
// Must 转 keywords 全命中，Any 转 matchKeywords 任一命中，条件为空时返回 nil
func buildFilterConditions(filter *Filter) *qdrant.Filter {
	if filter == nil {
		return nil
	}
	must := make([]*qdrant.Condition, 0, len(filter.Must)+len(filter.Any))
	for field, value := range filter.Must {
		must = append(must, qdrant.NewMatchKeyword(field, value))
	}
	for field, values := range filter.Any {
		if len(values) == 0 {
			continue
		}
		must = append(must, qdrant.NewMatchKeywords(field, values...))
	}
	if len(must) == 0 {
		return nil
	}
	return &qdrant.Filter{Must: must}
}

// pointID 把向量点 id 转成字符串，兼容 uuid 与数字两种 id
func pointID(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	if uuid := id.GetUuid(); uuid != "" {
		return uuid
	}
	if num := id.GetNum(); num != 0 {
		return strconv.FormatUint(num, 10)
	}
	return ""
}
