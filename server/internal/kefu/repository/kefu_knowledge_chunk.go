package repository

import (
	"admin/server/model"
	"admin/server/pkg/db"
)

// KefuKnowledgeChunkRepository 是知识库分块的数据访问；
// 分块内容落库便于重建与审计，向量库只存向量与 payload
type KefuKnowledgeChunkRepository struct{}

// NewKefuKnowledgeChunkRepository 创建知识库分块的数据访问
func NewKefuKnowledgeChunkRepository() *KefuKnowledgeChunkRepository {
	return &KefuKnowledgeChunkRepository{}
}

// CreateBatch 批量新增分块
func (r *KefuKnowledgeChunkRepository) CreateBatch(list []model.KefuKnowledgeChunk) error {
	if len(list) == 0 {
		return nil
	}
	return db.DB.Create(&list).Error
}

// ListByVectorIDs 按向量点 id 批量查询分块，检索命中后据此回填分块内容
func (r *KefuKnowledgeChunkRepository) ListByVectorIDs(vectorIDs []string) ([]model.KefuKnowledgeChunk, error) {
	if len(vectorIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuKnowledgeChunk
	err := db.DB.Model(&model.KefuKnowledgeChunk{}).
		Where("deleted = 0 and vector_id in ?", vectorIDs).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CountByDoc 统计指定文档未删除的分块数量，入库时据此判断是否需要清理旧索引
func (r *KefuKnowledgeChunkRepository) CountByDoc(docID int64) (int64, error) {
	var count int64
	err := db.DB.Model(&model.KefuKnowledgeChunk{}).
		Where("doc_id = ? and deleted = 0", docID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// RemoveByDoc 软删除指定文档的全部分块，重建索引时先清旧分块
func (r *KefuKnowledgeChunkRepository) RemoveByDoc(docID int64) error {
	return db.DB.Model(&model.KefuKnowledgeChunk{}).
		Where("doc_id = ? and deleted = 0", docID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
