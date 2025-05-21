package routes

import (
	"github.com/ciliverse/cilikube/api/v1/handlers" // Ensure correct path
	"github.com/gin-gonic/gin"
)

// RegisterSummaryRoutes 注册资源汇总相关路由
func RegisterSummaryRoutes(router *gin.RouterGroup, handler *handlers.SummaryHandler) {
	// 资源汇总路由
	summaryGroup := router.Group("/summary")
	{
		summaryGroup.GET("/resources", handler.GetResourceSummary)
		// *** ADD THIS LINE ***
		summaryGroup.GET("/backend-dependencies", handler.GetBackendDependencies) // Register the new handlers
	}
}

// If you have an authenticated version, add it there too if needed
/*
func RegisterSummaryRoutesWithAuth(router *gin.RouterGroup, handlers *handlers.SummaryHandler, authMiddleware ...gin.HandlerFunc) {
	summaryGroup := router.Group("/summary")
    // Apply middleware if needed
    // summaryGroup.Use(authMiddleware...)
	{
		summaryGroup.GET("/resources", handlers.GetResourceSummary)
        summaryGroup.GET("/backend-dependencies", handlers.GetBackendDependencies)
	}
}
*/
