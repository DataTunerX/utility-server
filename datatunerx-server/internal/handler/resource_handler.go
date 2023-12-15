package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"datatunerx-server/config"
	"datatunerx-server/pkg/k8s"
	"datatunerx-server/pkg/ray"

	corev1beta1 "github.com/DataTunerX/meta-server/api/core/v1beta1"
	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"

	"github.com/DataTunerX/utility-server/logging"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
)

// ResourceHandler struct contains necessary dependencies
type ResourceHandler struct {
	KubeClients k8s.KubernetesClients
	RayClients  ray.RayClient
}

// NewResourceHandler creates a new instance of ResourceHandler
func NewResourceHandler(kubeClients k8s.KubernetesClients, rayClients ray.RayClient) *ResourceHandler {
	return &ResourceHandler{
		KubeClients: kubeClients,
		RayClients:  rayClients,
	}
}

// Map resourceKind to resource names
var resourceKindMapping = map[string]string{
	"datasets": "datasets",
	"scorings": "scorings",
	// Add mappings for other resource types
}

// Handle requests to update resources
func (rh *ResourceHandler) UpdateResourceHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	resourceKind := c.Param("resourceKind")
	resourceName := c.Param("resourceName")
	group := c.Param("group")
	version := c.Param("version")
	kind := c.Param("kind")
	objName := c.Param("objName")

	fmt.Printf("Received request: namespace=%s, resourceKind=%s, resourceName=%s\n", namespace, resourceKind, resourceName)

	// Get dynamic client
	dynamicClient := rh.KubeClients.DynamicClient

	// Map resourceKind to the corresponding resource name
	resource, ok := resourceKindMapping[resourceKind]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid resourceKind: %s", resourceKind)})
		return
	}

	fmt.Printf("Mapped resourceKind %s to resource %s\n", resourceKind, resource)

	// Get data from the request
	var requestBody interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("Received JSON data: %v\n", requestBody)

	// Get GroupVersionResource for the corresponding resource object
	resourceGroupVersion := schema.GroupVersionResource{
		Group:    "extension.datatunerx.io",
		Version:  "v1beta1",
		Resource: resource,
	}

	// Get the resource object
	resourceObject, err := dynamicClient.Resource(resourceGroupVersion).Namespace(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to get %s resource: %v", resource, err)})
		return
	}

	fmt.Printf("Retrieved existing resource object: %+v\n", resourceObject)

	// Update the resource object's spec and status with retry mechanism
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {

		// Check if the request body is an array
		if subsets, isArray := requestBody.([]interface{}); isArray {
			// If it's an array, transform the request body
			tmpRequestBody := map[string]interface{}{"spec": map[string]interface{}{"datasetMetadata": map[string]interface{}{"datasetInfo": map[string]interface{}{"subsets": subsets}}}}
			// Convert requestBody to []byte
			requestBodyBytes, err := json.Marshal(tmpRequestBody)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal JSON: %v", err)})
				return err
			}
			_, updateErr := dynamicClient.Resource(resourceGroupVersion).Namespace(namespace).Patch(context.TODO(),
				resourceName,
				types.MergePatchType,
				requestBodyBytes,
				metav1.PatchOptions{},
			)
			return updateErr
		} else {
			// If it's a map, transform the request body and use UpdateStatus
			// tmpRequestBody := map[string]interface{}{"status": requestBody}
			resourceObject.Object["status"] = requestBody
			_, updateErr := dynamicClient.Resource(resourceGroupVersion).Namespace(namespace).UpdateStatus(context.TODO(),
				resourceObject,
				metav1.UpdateOptions{},
			)
			return updateErr
		}
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update %s resource: %v", resource, err)})
		return
	}

	toDeleteResourceGroupVersion := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: kind,
	}
	// Delete the specified object
	err = dynamicClient.Resource(toDeleteResourceGroupVersion).Namespace(namespace).Delete(context.TODO(), objName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete %s object: %v", objName, err)})
		return
	}
	// Return a success response
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s %s/%s updated successfully", resource, namespace, resourceName)})
}

// ListRayServices lists rayservices objects in the specified namespace with the given label selector
func (rh *ResourceHandler) ListRayServicesHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	// Specify the label selector
	labelSelector := config.GetInferenceServiceLabel()
	rayServicesList, err := rh.RayClients.Clientset.RayV1().RayServices(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list rayservices: %v", err)})
		return
	}

	// Return the list of rayservices objects
	c.JSON(http.StatusOK, rayServicesList)
}

// CreateRayServiceHandler 创建 Rayservice 对象的路由处理函数
func (rh *ResourceHandler) CreateRayServiceHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	// 从请求体中获取创建 Rayservice 所需的数据
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse request body: %v", err)})
		return
	}

	var image, llmPath, checkpointPath string
	llmCheckpoint, err := rh.GetLlmCheckpoint(requestBody["llmCheckpoint"].(string))
	if err != nil {
		logging.ZLogger.Errorf("Failed to get LlmCheckpoint: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get LlmCheckpoint: %v", err)})
	}
	if llmCheckpoint.Spec.CheckpointImage != nil {
		if llmCheckpoint.Spec.CheckpointImage.Name != nil {
			image = *llmCheckpoint.Spec.CheckpointImage.Name
		} else {
			logging.ZLogger.Error("LlmCheckpoint missing CheckpointImage.Name")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "LlmCheckpoint missing CheckpointImage.Name"})
			return
		}
		if llmCheckpoint.Spec.CheckpointImage.LLMPath != "" {
			llmPath = llmCheckpoint.Spec.CheckpointImage.LLMPath
		} else {
			logging.ZLogger.Error("LlmCheckpoint missing CheckpointImage.LLMPath")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "LlmCheckpoint missing CheckpointImage.LLMPath"})
			return
		}
		if llmCheckpoint.Spec.CheckpointImage.CheckPointPath != "" {
			checkpointPath = llmCheckpoint.Spec.CheckpointImage.CheckPointPath
		} else {
			logging.ZLogger.Error("LlmCheckpoint missing CheckpointImage.CheckPointPath")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "LlmCheckpoint missing CheckpointImage.CheckPointPath"})
			return
		}
	} else {
		logging.ZLogger.Error("LlmCheckpoint missing CheckpointImage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LlmCheckpoint missing CheckpointImage"})
		return
	}
	requestBody["image"] = image
	requestBody["llmPath"] = llmPath
	requestBody["checkpointPath"] = checkpointPath
	// 创建 Rayservice 对象
	rayService := rh.buildRayServiceObject(namespace, requestBody)

	// 使用 Rayservice 的 Client 进行创建
	createdRayService, err := rh.RayClients.Clientset.RayV1().RayServices(namespace).Create(context.TODO(), rayService, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create rayservice: %v", err)})
		return
	}

	// 返回创建成功的 Rayservice 对象
	c.JSON(http.StatusOK, createdRayService)
}

// buildRayServiceObject 用于构建 Rayservice 对象
func (rh *ResourceHandler) buildRayServiceObject(namespace string, data map[string]interface{}) *rayv1.RayService {
	// 根据你的数据结构构建 Rayservice 对象，以下是一个示例，你需要根据实际情况修改
	replicas := int32(1)
	gpu := float64(1)
	rayService := &rayv1.RayService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      data["name"].(string),
			Namespace: namespace,
			Labels: func() map[string]string {
				parts := strings.Split(config.GetInferenceServiceLabel(), "=")
				if len(parts) != 2 {
					return nil
				}
				return map[string]string{parts[0]: parts[1]}
			}(),
		},
		Spec: rayv1.RayServiceSpec{
			RayClusterSpec: rayv1.RayClusterSpec{
				HeadGroupSpec: rayv1.HeadGroupSpec{
					ServiceType:    v1.ServiceTypeNodePort,
					RayStartParams: map[string]string{"dashboard-host": "0.0.0.0", "num-gpus": "0"},
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Image: data["image"].(string),
									Name:  "ray-head",
									Ports: []v1.ContainerPort{
										{
											ContainerPort: 6379,
											Name:          "gcs-server",
											Protocol:      v1.ProtocolTCP,
										},
										{
											ContainerPort: 8265,
											Name:          "dashboard",
											Protocol:      v1.ProtocolTCP,
										},
										{
											ContainerPort: 10001,
											Name:          "client",
											Protocol:      v1.ProtocolTCP,
										},
										{
											ContainerPort: 8000,
											Name:          "serve",
											Protocol:      v1.ProtocolTCP,
										},
									},
									Resources: v1.ResourceRequirements{
										Limits: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("2000m"),
											v1.ResourceMemory: resource.MustParse("8Gi"),
										},
										Requests: v1.ResourceList{
											v1.ResourceCPU:    resource.MustParse("1000m"),
											v1.ResourceMemory: resource.MustParse("4Gi"),
										},
									},
								},
							},
						},
					},
				},
				RayVersion: "2.7.1",
				WorkerGroupSpecs: []rayv1.WorkerGroupSpec{
					{
						GroupName:      "worker",
						MaxReplicas:    &replicas,
						MinReplicas:    &replicas,
						RayStartParams: map[string]string{},
						Replicas:       &replicas,
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								NodeSelector: map[string]string{"nvidia.com/gpu": "present"},
								Containers: []v1.Container{
									{
										Image: data["image"].(string),
										Env: []v1.EnvVar{
											{
												Name:  "BASE_MODEL_DIR",
												Value: data["llmPath"].(string),
											},
											{
												Name:  "CHECKPOINT_DIR",
												Value: data["checkpointPath"].(string),
											},
										},
										Lifecycle: &v1.Lifecycle{
											PreStop: &v1.LifecycleHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "ray stop"},
												},
											},
										},
										Resources: v1.ResourceRequirements{
											Limits: v1.ResourceList{
												v1.ResourceCPU:    resource.MustParse("8000m"),
												v1.ResourceMemory: resource.MustParse("48Gi"),
												"nvidia.com/gpu":  resource.MustParse("1"),
											},
											Requests: v1.ResourceList{
												v1.ResourceCPU:    resource.MustParse("1000m"),
												v1.ResourceMemory: resource.MustParse("48Gi"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			ServeDeploymentGraphSpec: rayv1.ServeDeploymentGraphSpec{
				ServeConfigSpecs: []rayv1.ServeConfigSpec{
					{
						Name:        "LlamaDeployment",
						NumReplicas: &replicas,
						RayActorOptions: rayv1.RayActorOptionSpec{
							NumGpus: &gpu,
						},
					},
				},
				ImportPath: "inference.deployment",
				RuntimeEnv: `working_dir: file:///home/inference/inference.zip`,
			},
			ServeService: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   data["name"].(string) + "-service",
					Labels: map[string]string{"app": "inference"},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:       "serve",
							Port:       8000,
							TargetPort: intstr.FromInt(8000),
							Protocol:   v1.ProtocolTCP,
						},
					},
					Selector: map[string]string{"ray.io/node-type": "head"},
				},
			},
		},
		// 其他 Rayservice 对象数据，根据需要添加
	}

	return rayService
}

func (rh *ResourceHandler) GetLlmCheckpoint(name string) (corev1beta1.LLMCheckpoint, error) {
	llmCheckpointGroupVersion := schema.GroupVersionResource{
		Group:    "core.datatunerx.io",
		Version:  "v1beta1",
		Resource: "LLMCheckpoint",
	}
	llmCheckpointUnstructured, err := rh.KubeClients.DynamicClient.Resource(llmCheckpointGroupVersion).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		logging.ZLogger.Errorf("Failed to get LLMCheckpoint resource: %v", err)
		return corev1beta1.LLMCheckpoint{}, err
	}
	var llmCheckpoint corev1beta1.LLMCheckpoint
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(llmCheckpointUnstructured.Object, &llmCheckpoint)
	if err != nil {
		logging.ZLogger.Errorf("Failed to convert Unstructured to LLMCheckpoint: %v", err)
		return corev1beta1.LLMCheckpoint{}, err
	}
	return llmCheckpoint, nil
}
