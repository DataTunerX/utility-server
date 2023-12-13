// internal/handler/inference_handler.go

package handler

import (
	"bytes"
	"datatunerx-server/config"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/DataTunerX/utility-server/logging"
	"github.com/gin-gonic/gin"
)

// InferenceHandler 是处理 /inference 的路由处理函数
func InferenceHandler(c *gin.Context) {
	logging.NewZapLogger(config.GetLevel())
	// 解析请求体
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to parse request body: %v", err)})
		return
	}

	// 获取 model 字段值
	model, ok := requestBody["model"].(string)
	if !ok {
		c.JSON(400, gin.H{"error": "Missing or invalid 'model' field in the request body"})
		return
	}

	// 解析 model 字段值，形式为 "servicename.namespace"
	modelParts := parseModelField(model)
	if modelParts == nil {
		c.JSON(400, gin.H{"error": "Invalid 'model' field format"})
		return
	}

	// 构建目标服务地址
	targetServiceURL := fmt.Sprintf("http://%s.svc.cluster.local/chat/completions", model)

	// 发起转发请求
	resp, err := forwardRequest(targetServiceURL, requestBody)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to forward request: %v", err)})
		return
	}

	// 返回目标服务的响应
	c.JSON(resp.StatusCode, resp.Body)
}

// parseModelField 解析 model 字段值，返回服务名和命名空间
func parseModelField(model string) []string {
	return strings.Split(model, ".")
}

// forwardRequest 发起转发请求
func forwardRequest(targetURL string, requestBody map[string]interface{}) (*http.Response, error) {
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
