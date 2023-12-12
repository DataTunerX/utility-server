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

	// Set up routes
	router.POST("/apis/util.datatunerx.io/v1beta1/namespaces/:namespace/:resourceKind/:resourceName/:group/:version/:kind/:objName", handler.NewResourceHandler(kubeClients).UpdateResourceHandler)

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
