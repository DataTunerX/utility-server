package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesClients contains two clients, *kubernetes.Clientset and dynamic.Interface
type KubernetesClients struct {
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
}

// InitKubeClient initializes Kubernetes clients
func InitKubeClient() KubernetesClients {
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
