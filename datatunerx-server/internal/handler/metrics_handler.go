package handler

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"datatunerx-server/pkg/k8s"
)

type FinetuneMetricsHandler struct {
	KubeClients k8s.KubernetesClients
}

func NewFinetuneMetricsHandler(kubeClients k8s.KubernetesClients) *FinetuneMetricsHandler {
	return &FinetuneMetricsHandler{KubeClients: kubeClients}
}

type Metrics struct {
	CurrentSteps   int     `json:"current_steps"`
	TotalSteps     int     `json:"total_steps"`
	Loss           float64 `json:"loss,omitempty"`
	LearningRate   float64 `json:"learning_rate,omitempty"`
	Epoch          float64 `json:"epoch"`
	EvalLoss       float64 `json:"eval_loss,omitempty"`
	EvalPerplexity float64 `json:"eval_perplexity,omitempty"`
}

type SeparatedMetrics struct {
	TrainMetrics []*Metrics `json:"train_metrics"`
	EvalMetrics  []*Metrics `json:"eval_metrics"`
}

type FinetuneMetrics struct {
	FinetuneName string            `json:"finetune_name"`
	Metrics      *SeparatedMetrics `json:"metrics"`
}

// GetFinetuneMetrics is the handler for GET finetune metrics
func (h *FinetuneMetricsHandler) GetFinetuneMetrics(c *gin.Context) {
	namespace := c.Param("namespace")
	finetuneNames := c.QueryArray("finetune_name")

	fmt.Printf("Received request: namespace=%s, finetuneNames=%s\n", namespace, finetuneNames)

	if len(finetuneNames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing query parameter 'finetune_name'"})
		return
	}
	// Convert finetuneNames to a map for efficient lookup
	finetuneNamesMap := make(map[string]struct{})
	for _, name := range finetuneNames {
		fmt.Printf("name: %s\n", name)
		if name != "" {
			finetuneNamesMap[name] = struct{}{}
		}
	}

	dynamicClient := h.KubeClients.DynamicClient
	// Get GroupVersionResource for the corresponding resource object
	resourceGroupVersion := schema.GroupVersionResource{
		Group:    "finetune.datatunerx.io",
		Version:  "v1beta1",
		Resource: "finetunes",
	}

	finetuneInstances, err := dynamicClient.Resource(resourceGroupVersion).Namespace(namespace).List(c, metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to get finetuneInstances: %v", err)})
		return
	}

	var finetuneUIDs []string
	finetuneUIDNameMap := make(map[string]string)
	finetuneInstancesMap := make(map[string]unstructured.Unstructured)

	for _, instance := range finetuneInstances.Items {
		instanceName := instance.GetName()
		if _, ok := finetuneNamesMap[instanceName]; ok {
			uid := string(instance.GetUID())
			finetuneInstancesMap[instanceName] = instance
			finetuneUIDNameMap[uid] = instanceName
			finetuneUIDs = append(finetuneUIDs, uid)
		}
	}

	fmt.Printf("finetuneNameUidMap: %v\n", finetuneUIDNameMap)

	prometheusClient, err := newPrometheusClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	r := v1.Range{
		Start: time.Now().Add(-time.Hour * 24 * 7),
		End:   time.Now(),
		Step:  time.Minute,
	}

	queryUIDs := strings.Join(finetuneUIDs, "|")
	query := fmt.Sprintf("train_metrics{uid=~\"%s\"} or eval_metrics{uid=~\"%s\"}", queryUIDs, queryUIDs)

	fmt.Printf("query: %s\n", query)

	val, _, err := prometheusClient.QueryRange(c, query, r)
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	matrixTagged := val.(model.Matrix)

	fmt.Printf("result: %v\n", matrixTagged)

	rawMetrics := make(map[string]*SeparatedMetrics)
	for _, stream := range matrixTagged {
		uid := string(stream.Metric["uid"])
		metricsType := string(stream.Metric["__name__"])
		switch metricsType {
		case "train_metrics":
			if _, ok := rawMetrics[uid]; !ok {
				rawMetrics[uid] = &SeparatedMetrics{}
			}
			rawMetrics[uid].TrainMetrics = append(rawMetrics[uid].TrainMetrics, &Metrics{
				CurrentSteps: func() int {
					val, err := strconv.Atoi(string(stream.Metric["current_steps"]))
					if err != nil {
						val = 0
					}
					return val
				}(),
				TotalSteps: func() int {
					val, err := strconv.Atoi(string(stream.Metric["total_steps"]))
					if err != nil {
						val = 0
					}
					return val
				}(),
				Loss: func() float64 {
					val, err := strconv.ParseFloat(string(stream.Metric["loss"]), 64)
					if err != nil {
						val = 0
					}
					return val
				}(),
				LearningRate: func() float64 {
					val, err := strconv.ParseFloat(string(stream.Metric["learning_rate"]), 64)
					if err != nil {
						val = 0
					}
					return val
				}(),
				Epoch: func() float64 {
					val, err := strconv.ParseFloat(string(stream.Metric["epoch"]), 64)
					if err != nil {
						val = 0
					}
					return val
				}(),
			})

		case "eval_metrics":
			if _, ok := rawMetrics[uid]; !ok {
				rawMetrics[uid] = &SeparatedMetrics{}
			}
			rawMetrics[uid].EvalMetrics = append(rawMetrics[uid].EvalMetrics, &Metrics{
				CurrentSteps: func() int {
					val, err := strconv.Atoi(string(stream.Metric["current_steps"]))
					if err != nil {
						val = 0
					}
					return val
				}(),
				TotalSteps: func() int {
					val, err := strconv.Atoi(string(stream.Metric["total_steps"]))
					if err != nil {
						val = 0
					}
					return val
				}(),
				EvalLoss: func() float64 {
					val, err := strconv.ParseFloat(string(stream.Metric["eval_loss"]), 64)
					if err != nil {
						val = 0
					}
					return val
				}(),
				EvalPerplexity: func() float64 {
					val, err := strconv.ParseFloat(string(stream.Metric["eval_perplexity"]), 64)
					if err != nil {
						val = 0
					}
					return val
				}(),
				Epoch: func() float64 {
					val, err := strconv.ParseFloat(string(stream.Metric["epoch"]), 64)
					if err != nil {
						val = 0
					}
					return val
				}(),
			})
		}
	}

	finetuneMetrics := make([]*FinetuneMetrics, 0, len(rawMetrics))
	for uid, metrics := range rawMetrics {

		// 对 TrainMetrics 进行排序
		sort.Slice(metrics.TrainMetrics, func(i, j int) bool {
			return metrics.TrainMetrics[i].CurrentSteps < metrics.TrainMetrics[j].CurrentSteps
		})

		// 对 EvalMetrics 进行排序
		sort.Slice(metrics.EvalMetrics, func(i, j int) bool {
			return metrics.EvalMetrics[i].CurrentSteps < metrics.EvalMetrics[j].CurrentSteps
		})

		finetuneMetrics = append(finetuneMetrics, &FinetuneMetrics{
			FinetuneName: finetuneUIDNameMap[uid],
			Metrics:      metrics,
		})
	}
	c.JSON(http.StatusOK, finetuneMetrics)
}

func getPrometheusAPIURL() string {
	prometheusAPIURL := os.Getenv("PROMETHEUS_API_URL")
	if prometheusAPIURL == "" {
		prometheusAPIURL = "http://localhost:9090"
	}
	return prometheusAPIURL
}

func newPrometheusClient() (v1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: getPrometheusAPIURL(),
	})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}
