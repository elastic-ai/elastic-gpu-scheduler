package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/controller"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/metrics"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/routes"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/scheduler"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils/signals"

	"github.com/julienschmidt/httprouter"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	log "k8s.io/klog/v2"
)

const RecommendedKubeConfigPathEnv = "KUBECONFIG"

var (
	clientset         *kubernetes.Clientset
	resyncPeriod      = 30 * time.Second
	PriorityAlgorithm string
)

func initKubeClient() {
	kubeConfig := ""
	if len(os.Getenv(RecommendedKubeConfigPathEnv)) > 0 {
		// use the current context in kubeconfig
		// This is very useful for running locally.
		kubeConfig = os.Getenv(RecommendedKubeConfigPathEnv)
	}

	// Get kubernetes config.
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// create the clientset
	clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to init rest config due to %v", err)
	}
}

func InitFlag() {
	flag.StringVar(&PriorityAlgorithm, "priority", "binpack", "priority algorithm, binpack/spread/random")
}

func main() {
	InitFlag()

	log.InitFlags(nil)
	flag.Parse()

	log.Info("Priority algorithm is ", PriorityAlgorithm)

	threadness := StringToInt(os.Getenv("THREADNESS"))

	initKubeClient()
	port := os.Getenv("PORT")
	if _, err := strconv.Atoi(port); err != nil {
		port = "39999"
	}

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()

	informerFactory := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	schudulerController, err := controller.NewController(clientset, informerFactory, stopCh)
	if err != nil {
		log.Fatalf("Failed to start due to %v", err)
		return
	}

	metrics.Register()

	go schudulerController.Run(threadness, stopCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	predicate := scheduler.NewNanoGPUPredicate(ctx, clientset, schudulerController.GetDealer())
	prioritize := scheduler.NewNanoGPUPrioritize(ctx, clientset, schudulerController.GetDealer())
	bind := scheduler.NewNanoGPUBind(ctx, clientset, schudulerController.GetDealer())

	router := httprouter.New()
	routes.AddPProf(router)
	routes.AddVersion(router)
	routes.AddMetrics(router)

	routes.AddPredicate(router, predicate)
	routes.AddPrioritize(router, prioritize)
	routes.AddBind(router, bind)

	log.V(3).Infof("server starting on the port :%s", port)

	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}

func StringToInt(sThread string) int {
	thread := 1

	return thread
}
