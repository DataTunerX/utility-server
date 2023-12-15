package ray

import (
	"fmt"
	"os"

	"github.com/ray-project/kuberay/ray-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// RayClient contains the Ray V1 client
type RayClient struct {
	Clientset *versioned.Clientset
}

// InitRayClient initializes the Ray V1 client
func InitRayClient() (RayClient, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	var config *rest.Config
	var err error

	// If running inside a Kubernetes cluster, use in-cluster config
	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			return RayClient{}, fmt.Errorf("failed to get in-cluster config: %v", err)
		}
		fmt.Println("Using in-cluster config")
	} else {
		// If running locally, use the provided kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return RayClient{}, fmt.Errorf("failed to build kubeconfig from file: %v", err)
		}
		fmt.Printf("Using kubeconfig file: %s\n", kubeconfig)
	}

	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return RayClient{}, fmt.Errorf("failed to create Ray V1 client: %v", err)
	}

	return RayClient{
		Clientset: clientset,
	}, nil
}
