package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuKnowledgeService 是知识库的业务处理
var kefuKnowledgeService = service.NewKefuKnowledgeService()

// handleKnowledgeError 按错误类型输出知识库相关的响应
func handleKnowledgeError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrKnowledgeNotFound),
		errors.Is(err, service.ErrKnowledgeFileNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrKnowledgeCategoryRequired),
		errors.Is(err, service.ErrKnowledgeCategoryInvalid),
		errors.Is(err, service.ErrKnowledgeTitleRequired),
		errors.Is(err, service.ErrKnowledgeFileRequired),
		errors.Is(err, service.ErrKnowledgeSourceInvalid),
		errors.Is(err, service.ErrKnowledgeUnsupportedExt),
		errors.Is(err, service.ErrKnowledgeContentEmpty),
		errors.Is(err, service.ErrKnowledgeQueryRequired),
		errors.Is(err, service.ErrRagUnavailable):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuKnowledgePage 分页查询知识库文档
func KefuKnowledgePage(c *gin.Context) {
	var param model.KefuKnowledgeParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuKnowledgeService.Page(param)
	if err != nil {
		common.ServerError(c, "查询知识库失败")
		return
	}
	common.Page(c, list, count)
}

// KefuKnowledgeAdd 新增知识库文档，上传后异步向量化
func KefuKnowledgeAdd(c *gin.Context) {
	var form model.KefuKnowledgeForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	userID, _ := common.CurrentUserID(c)
	record, err := kefuKnowledgeService.Add(form, userID)
	if err != nil {
		handleKnowledgeError(c, err, "添加知识库文档失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功，正在解析入库", record)
}

// KefuKnowledgeUpdate 修改知识库文档的标题与分类
func KefuKnowledgeUpdate(c *gin.Context) {
	var form model.KefuKnowledgeForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.DocID == 0 {
		common.BadRequest(c, "知识库文档id不能为空")
		return
	}

	if err := kefuKnowledgeService.Update(form); err != nil {
		handleKnowledgeError(c, err, "修改知识库文档失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功，请重建索引后生效", nil)
}

// KefuKnowledgeRemove 删除知识库文档
func KefuKnowledgeRemove(c *gin.Context) {
	docID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "知识库文档id不正确")
		return
	}

	if err := kefuKnowledgeService.Remove(docID); err != nil {
		handleKnowledgeError(c, err, "删除知识库文档失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// KefuKnowledgeReindex 重建知识库文档索引
func KefuKnowledgeReindex(c *gin.Context) {
	docID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "知识库文档id不正确")
		return
	}

	record, err := kefuKnowledgeService.Reindex(docID)
	if err != nil {
		handleKnowledgeError(c, err, "重建知识库索引失败")
		return
	}
	common.SuccessWithMessage(c, "已提交重建，正在解析入库", record)
}

// KefuKnowledgeSearch 知识库召回测试
func KefuKnowledgeSearch(c *gin.Context) {
	var form model.KefuKnowledgeSearchForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	list, err := kefuKnowledgeService.Search(c.Request.Context(), form)
	if err != nil {
		handleKnowledgeError(c, err, "检索知识库失败")
		return
	}
	common.Success(c, list)
}
