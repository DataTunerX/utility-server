package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

// KubernetesClients contains two clients, *kubernetes.Clientset and dynamic.Interface
type KubernetesClients struct {
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
}

// Map resourceKind to resource names
var resourceKindMapping = map[string]string{
	"datasets": "datasets",
	"scorings": "scorings",
	// Add mappings for other resource types
}

var kubeClients KubernetesClients

func main() {
	// Initialize Kubernetes clients
	kubeClients = initKubeClient()

	// Initialize Gin Engine
	router := gin.Default()

	// Set up routes
	router.POST("/apis/util.datatunerx.io/v1beta1/namespaces/:namespace/:resourceKind/:resourceName", updateResourceHandler)

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

// Initialize Kubernetes clients
func initKubeClient() KubernetesClients {
	kubeconfig := os.Getenv("KUBECONFIG")
	var config *rest.Config
	var err error

	// If running inside a Kubernetes cluster, use in-cluster config
	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("Using in-cluster config")
	} else {
		// If running locally, use the provided kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("Using kubeconfig file: %s\n", kubeconfig)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return KubernetesClients{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
	}
}

// Handle requests to update resources
func updateResourceHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	resourceKind := c.Param("resourceKind")
	resourceName := c.Param("resourceName")
	objName := c.Param("objName")

	fmt.Printf("Received request: namespace=%s, resourceKind=%s, resourceName=%s\n", namespace, resourceKind, resourceName)

	// Get dynamic client
	dynamicClient := kubeClients.DynamicClient

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
			requestBody = map[string]interface{}{"spec": map[string]interface{}{"datasetMetadata": map[string]interface{}{"datasetInfo": map[string]interface{}{"subsets": subsets}}}}
			// Convert requestBody to []byte
			requestBodyBytes, err := json.Marshal(requestBody)
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
			requestBody = map[string]interface{}{"status": requestBody}
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

	// Delete the specified object
	err = dynamicClient.Resource(resourceGroupVersion).Namespace(namespace).Delete(context.TODO(), objName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete %s object: %v", objName, err)})
		return
	}
	// Return a success response
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s %s/%s updated successfully", resource, namespace, resourceName)})
}
