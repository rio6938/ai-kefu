package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
)

// parseID 把路径参数解析为数字 id
func parseID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// parseIDs 解析请求体中的 id 数组
func parseIDs(c *gin.Context) ([]int64, bool) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		common.BadRequest(c, "请选择要操作的数据")
		return nil, false
	}
	return ids, true
}
