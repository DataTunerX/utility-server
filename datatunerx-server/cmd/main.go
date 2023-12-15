package main

import (
	"os"

	"datatunerx-server/internal/handler"
	"datatunerx-server/pkg/k8s"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize Kubernetes clients
	kubeClients := k8s.InitKubeClient()

	// Initialize Gin Engine
	router := gin.Default()

	apiGroup := router.Group("/apis/util.datatunerx.io/v1beta1")

	namespaceGroup := apiGroup.Group("/namespaces/:namespace")
	// plugin webhook routes
	resourceUpdate := namespaceGroup.Group("/:resourceKind/:resourceName")
	{
		resourceUpdate.POST("/:group/:version/:kind/:objName", handler.NewResourceHandler(kubeClients).UpdateResourceHandler)
	}
	// inference service routes
	inferenceService := namespaceGroup.Group("/services")
	{
		inferenceService.GET("", handler.NewResourceHandler(kubeClients).ListRayServices)
	}

	// inference proxy routes
	inferenceProxy := apiGroup.Group("/inference")
	{
		inferenceProxy.POST("/chat", handler.InferenceHandler)
	}

	// finetune metrics routes
	finetuneMetrics := namespaceGroup.Group("/finetune/metrics")
	{
		finetuneMetrics.GET("", handler.NewFinetuneMetricsHandler(kubeClients).GetFinetuneMetrics)
	}

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
