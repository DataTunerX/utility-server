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
	c.JSON(200, gin.H{
		"output":      resp.Output,
		"tokenLength": resp.TokenLength,
		"elaspedTime": resp.ElapsedTime,
		"tokenPerSec": resp.TokenPerSec,
	})
}

// forwardRequest 发起转发请求
func forwardRequest(targetURL string, requestBody InferenceBody) (InferenceProcessedResponse, error) {
	// 将请求体转换为 JSON 字符串
	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		logging.ZLogger.Errorf("Failed to marshal JSON request body: %v", err)
		return InferenceProcessedResponse{}, err
	}

	// 发起 POST 请求
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		logging.ZLogger.Errorf("Failed to forward request: %v", err)
		return InferenceProcessedResponse{}, err
	}
	defer resp.Body.Close()

	// 解析响应体
	var response InferenceResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&response); err != nil {
		logging.ZLogger.Errorf("Failed to decode JSON response: %v", err)
		return InferenceProcessedResponse{}, err
	}

	// 处理响应数据
	output := response.Choices[0].Message.Content
	tokenLength := response.Usage.TotalTokens
	elapsedTIme := response.Usage.ElapsedTIme
	tokenPerSec := response.Usage.TokenPerSec

	// 构造处理后的响应数据
	processedResponse := InferenceProcessedResponse{
		Output:      output,
		TokenLength: tokenLength,
		ElapsedTime: elapsedTIme,
		TokenPerSec: tokenPerSec,
	}

	return processedResponse, nil
}

type InferenceBody struct {
	Model    string                 `json:"model"`
	Messages []InferenceBodyMessage `json:"messages"`
}

type InferenceBodyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InferenceUsage struct {
	PromptTokens     string `json:"prompt_tokens"`
	CompletionTokens string `json:"completion_tokens"`
	TotalTokens      string `json:"total_tokens"`
	ElapsedTIme      string `json:"elasped_time"`
	TokenPerSec      string `json:"token_per_sec"`
}

type InferenceChoice struct {
	Index        int                  `json:"index"`
	Message      InferenceBodyMessage `json:"message"`
	FinishReason string               `json:"finish_reason"`
}

type InferenceResponse struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	Created           int64             `json:"created"`
	Model             string            `json:"model"`
	SystemFingerprint string            `json:"system_fingerprint"`
	Choices           []InferenceChoice `json:"choices"`
	Usage             InferenceUsage    `json:"usage"`
}

type InferenceProcessedResponse struct {
	Output      string `json:"output"`
	TokenLength string `json:"tokenLength"`
	ElapsedTime string `json:"elapsedTime"`
	TokenPerSec string `json:"tokenPerSec"`
}
