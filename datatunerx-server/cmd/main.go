package main

import (
	"os"

	"datatunerx-server/config"
	"datatunerx-server/internalp/handler"
	"datatunerx-server/pkg/k8s"
	"datatunerx-server/pkg/ray"
	"datatunerx-server/pkg/s3"

	"github.com/DataTunerX/utility-server/logging"
	"github.com/gin-gonic/gin"
)

func main() {
	logging.NewZapLogger(config.GetLevel())
	// Initialize Kubernetes clients
	kubeClients := k8s.InitKubeClient()
	// Initialize Ray clients
	rayClients, err := ray.InitRayClient()
	if err != nil {
		logging.ZLogger.Errorf("Error initializing ray client: %v", err)
	}
	// Initialize S3 client
	s3Client, err := s3.NewS3Client(config.GetS3ServiceEndpoint(), config.GetS3ServiceAccessKey(), config.GetS3ServiceSecretKey(), config.GetS3ServiceUseSSL())
	if err != nil {
		logging.ZLogger.Errorf("Error initializing s3 client: %v", err)
	}
	// Initialize Gin Engine
	router := gin.Default()

	apiGroup := router.Group("/apis/util.datatunerx.io/v1beta1")
	namespaceGroup := apiGroup.Group("/namespaces/:namespace")

	// plugin webhook routes
	resourceUpdate := namespaceGroup.Group("/:resourceKind/:resourceName")
	{
		resourceUpdate.POST("/:group/:version/:kind/:objName", handler.NewResourceHandler(kubeClients, rayClients).UpdateResourceHandler)
	}
	// inference service routes
	inferenceService := namespaceGroup.Group("/services")
	{
		inferenceService.GET("", handler.NewResourceHandler(kubeClients, rayClients).ListRayServicesHandler)
		inferenceService.POST("", handler.NewResourceHandler(kubeClients, rayClients).CreateRayServiceHandler)
	}
	// inference proxy routes
	inferenceProxy := namespaceGroup.Group("/services/:serviceName/inference")
	{
		inferenceProxy.POST("/chat", handler.NewInferenceHandler(kubeClients, rayClients).InferenceChatHandler)
	}

	// finetune metrics routes
	finetuneMetrics := namespaceGroup.Group("/finetune/metrics")
	{
		finetuneMetrics.GET("", handler.NewFinetuneMetricsHandler(kubeClients).GetFinetuneMetrics)
	}

	// File upload route
	fileUpload := apiGroup.Group("/upload")
	{
		fileUpload.POST("", handler.NewUploadHandler(s3Client).UploadFile)
	}

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
