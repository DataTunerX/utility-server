package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
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

	// Get dynamic client
	dynamicClient := kubeClients.DynamicClient

	// Map resourceKind to the corresponding resource name
	resource, ok := resourceKindMapping[resourceKind]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid resourceKind: %s", resourceKind)})
		return
	}

	// Get data from the request
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	// Update the resource object's spec and status with retry mechanism
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Use DeepCopy to prevent modification of the original object during retry
		objCopy := resourceObject.DeepCopy()

		// Map data from the request body to the DeepCopy of the resource object
		if err := c.ShouldBindJSON(&objCopy.Object); err != nil {
			return err
		}

		_, updateErr := dynamicClient.Resource(resourceGroupVersion).Namespace(namespace).Update(context.TODO(), objCopy, metav1.UpdateOptions{})
		return updateErr
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update %s resource: %v", resource, err)})
		return
	}

	// Return a success response
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s %s/%s updated successfully", resource, namespace, resourceName)})
}
