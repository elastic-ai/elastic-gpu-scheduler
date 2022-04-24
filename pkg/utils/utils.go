package utils

import (
	"bytes"
	"elasticgpu.io/elastic-gpu/clientset/versioned"
	"encoding/gob"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"math"
	"os"
)

func DeepCopy(dst, src interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(dst)
}

func CloneInts(array []int) []int {
	ans := make([]int, len(array), cap(array))
	copy(ans, array)
	return ans
}

func Variance(value []float64) float64 {
	if len(value) == 1 {
		return 0.0
	}
	sum := 0.0
	for _, i := range value {
		sum += i
	}
	avg := sum / float64(len(value))
	res := 0.0
	for _, i := range value {
		res += math.Pow(i-avg, 2)
	}
	return res / float64(len(value)-1)
}

func InitKubeClientset(kubeconf string) (clientset *kubernetes.Clientset, egpuClientset *versioned.Clientset, err error) {
	kubeConfig := ""
	if kubeconf != "" {
		kubeConfig = kubeconf
	} else if len(os.Getenv(RecommendedKubeConfigPathEnv)) > 0 {
		// use the current context in kubeconfig
		// This is very useful for running locally.
		kubeConfig = os.Getenv(RecommendedKubeConfigPathEnv)
	}

	// Get kubernetes config.
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// create the clientset
	clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("Failed to init clientset due to %v", err)
	}
	egpuClientset, err = versioned.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("Failed to init egpu clientset due to %v", err)
	}

	return
}
