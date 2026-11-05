package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuKnowledgeCategoryService 是文档库分类的业务处理
var kefuKnowledgeCategoryService = service.NewKefuKnowledgeCategoryService()

// handleKnowledgeCategoryError 按错误类型输出文档库分类相关的响应
func handleKnowledgeCategoryError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrCategoryNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrCategoryNameRequired),
		errors.Is(err, service.ErrCategoryNameExists),
		errors.Is(err, service.ErrCategoryStatusInvalid),
		errors.Is(err, service.ErrCategoryInUse):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuKnowledgeCategoryPage 分页查询文档库分类
func KefuKnowledgeCategoryPage(c *gin.Context) {
	var param model.KefuKnowledgeCategoryParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuKnowledgeCategoryService.Page(param)
	if err != nil {
		common.ServerError(c, "查询文档库分类失败")
		return
	}
	common.Page(c, list, count)
}

// KefuKnowledgeCategoryList 查询启用中的文档库分类
func KefuKnowledgeCategoryList(c *gin.Context) {
	list, err := kefuKnowledgeCategoryService.List()
	if err != nil {
		common.ServerError(c, "查询文档库分类失败")
		return
	}
	common.Success(c, list)
}

// KefuKnowledgeCategoryAdd 新增文档库分类
func KefuKnowledgeCategoryAdd(c *gin.Context) {
	var form model.KefuKnowledgeCategoryForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuKnowledgeCategoryService.Add(form); err != nil {
		handleKnowledgeCategoryError(c, err, "添加文档库分类失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuKnowledgeCategoryUpdate 修改文档库分类
func KefuKnowledgeCategoryUpdate(c *gin.Context) {
	var form model.KefuKnowledgeCategoryForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.CategoryID == 0 {
		common.BadRequest(c, "分类id不能为空")
		return
	}

	if err := kefuKnowledgeCategoryService.Update(form); err != nil {
		handleKnowledgeCategoryError(c, err, "修改文档库分类失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuKnowledgeCategoryUpdateStatus 修改文档库分类状态
func KefuKnowledgeCategoryUpdateStatus(c *gin.Context) {
	var form model.KefuKnowledgeCategoryStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.CategoryID == 0 {
		common.BadRequest(c, "分类id不能为空")
		return
	}

	if err := kefuKnowledgeCategoryService.UpdateStatus(form.CategoryID, form.Status); err != nil {
		handleKnowledgeCategoryError(c, err, "修改文档库分类状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuKnowledgeCategoryRemove 删除文档库分类
func KefuKnowledgeCategoryRemove(c *gin.Context) {
	categoryID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "文档库分类id不正确")
		return
	}

	if err := kefuKnowledgeCategoryService.Remove(categoryID); err != nil {
		handleKnowledgeCategoryError(c, err, "删除文档库分类失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
