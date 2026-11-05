package common

import "github.com/gin-gonic/gin"

// 登录用户信息在 gin.Context 中的键名
const (
	contextUserID   = "userId"
	contextUsername = "username"
)

// SetCurrentUser 把登录用户信息写入上下文
func SetCurrentUser(c *gin.Context, userID int64, username string) {
	c.Set(contextUserID, userID)
	c.Set(contextUsername, username)
}

// CurrentUserID 读取当前登录用户的id
func CurrentUserID(c *gin.Context) (int64, bool) {
	value, ok := c.Get(contextUserID)
	if !ok {
		return 0, false
	}
	userID, ok := value.(int64)
	if !ok || userID == 0 {
		return 0, false
	}
	return userID, true
}

// CurrentUsername 读取当前登录用户的账号
func CurrentUsername(c *gin.Context) string {
	value, ok := c.Get(contextUsername)
	if !ok {
		return ""
	}
	username, _ := value.(string)
	return username
}
