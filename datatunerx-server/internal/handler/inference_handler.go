package handler

import (
	"github.com/gin-gonic/gin"
)

// ValidateServicesQueryParam 是中间件函数，用于验证 services 查询参数
func ValidateServicesQueryParam(c *gin.Context) {
	services := c.QueryArray("services")

	// 检查 services 是否是非空数组
	if len(services) == 0 {
		c.JSON(400, gin.H{"error": "Query parameter 'services' is required and must be a non-empty array"})
		c.Abort()
		return
	}

	// 在上下文中设置验证后的参数，以便后续的处理函数使用
	c.Set("services", services)

	// 继续执行后续处理函数
	c.Next()
}
