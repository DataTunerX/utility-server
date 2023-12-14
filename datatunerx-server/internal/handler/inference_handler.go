// internal/handler/inference_handler.go

package handler

import (
	"bytes"
	"context"
	"datatunerx-server/config"
	"datatunerx-server/pkg/k8s"
	"datatunerx-server/pkg/ray"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DataTunerX/utility-server/logging"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InferenceHandler struct {
	KubeClients k8s.KubernetesClients
	RayClients  ray.RayClient
}

// NewResourceHandler creates a new instance of ResourceHandler
func NewInferenceHandler(kubeClients k8s.KubernetesClients, rayClients ray.RayClient) *InferenceHandler {
	return &InferenceHandler{
		KubeClients: kubeClients,
		RayClients:  rayClients,
	}
}

// InferenceHandler 是处理 /inference 的路由处理函数
func (Ih *InferenceHandler) InferenceChatHandler(c *gin.Context) {
	logging.NewZapLogger(config.GetLevel())
	namespace := c.Param("namespace")
	rayServiceName := c.Param("serviceName")

	// Fetch rayservice object details
	rayClient := Ih.RayClients
	rayserviceObj, err := rayClient.Clientset.RayV1().RayServices(namespace).Get(context.TODO(), rayServiceName, metav1.GetOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to get rayservice: %v", err)})
		return
	}
	serviceName := rayserviceObj.Spec.ServeService.Name

	// 解析请求体
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to parse request body: %v", err)})
		return
	}

	// 获取 model 字段值
	input, ok := requestBody["input"].(string)
	transferBody := InferenceBody{
		Model: "serviceName",
		Messages: []InferenceBodyMessage{
			{
				Role:    "user",
				Content: input,
			},
		},
	}
	if !ok {
		c.JSON(400, gin.H{"error": "Missing or invalid 'model' field in the request body"})
		return
	}

	// 构建目标服务地址
	targetServiceURL := fmt.Sprintf("http://%s.%s.svc.cluster.local/chat/completions", serviceName, namespace)

	// 发起转发请求
	resp, err := forwardRequest(targetServiceURL, transferBody)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to forward request: %v", err)})
		return
	}

	// 返回目标服务的响应
	c.JSON(resp.StatusCode, resp.Body)
}

// forwardRequest 发起转发请求
func forwardRequest(targetURL string, requestBody InferenceBody) (*http.Response, error) {
	// 将请求体转换为 JSON 字符串
	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		logging.ZLogger.Errorf("Failed to marshal JSON request body: %v", err)
		return nil, err
	}

	// 发起 POST 请求
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		logging.ZLogger.Errorf("Failed to forward request: %v", err)
		return nil, err
	}

	return resp, nil
}

type InferenceBody struct {
	Model    string                 `json:"model"`
	Messages []InferenceBodyMessage `json:"messages"`
}

type InferenceBodyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
