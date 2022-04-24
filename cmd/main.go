package main

import (
	"context"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/controller"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/routes"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/scheduler"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/server"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils"
	"elasticgpu.io/elastic-gpu-scheduler/pkg/utils/signals"
	"flag"
	"github.com/julienschmidt/httprouter"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	PriorityAlgorithm string
	KubeconfigPath    string
	ResourceMode      string
)

func InitFlag() {
	flag.StringVar(&PriorityAlgorithm, "priority", "binpack", "priority algorithm, binpack/spread")
	flag.StringVar(&KubeconfigPath, "kubeconf", "kubeconf", "path to kubeconfig")
	flag.StringVar(&ResourceMode, "mode", "", "resource mode, pgpu/qgpu/gpushare")
}

func main() {
	InitFlag()
	klog.InitFlags(nil)
	flag.Parse()

	// init kubernetes clientset
	clientset, egpuClientset, err := utils.InitKubeClientset(KubeconfigPath)
	if err != nil {
		klog.Fatalf("failed to init kube client: %v", err)
	}

	// set up priority algorithm
	klog.Infof("priority algorithm: %s", PriorityAlgorithm)
	var rater scheduler.Rater
	switch PriorityAlgorithm {
	case utils.PrioritySpread:
		rater = &scheduler.Spread{}
	case utils.PriorityBinPack:
		rater = &scheduler.Binpack{}
	default:
		klog.Errorf("priority algorithm is not supported: %s", PriorityAlgorithm)
		return
	}

	config := scheduler.ElasticSchedulerConfig{
		Clientset:     clientset,
		EGPUClientset: egpuClientset,
		Rater:         rater,
	}

	schs, err := scheduler.BuildResourceSchedulers(strings.Split(ResourceMode, ","), config)
	if err != nil {
		klog.Fatalf("failed to build schedulers: %s", err.Error())
	}
	config.RegisteredSchedulers = schs

	threadness := StringToInt(os.Getenv("THREADNESS"))
	port := os.Getenv("PORT")
	if _, err := strconv.Atoi(port); err != nil {
		port = "39999"
	}
	stopCh := signals.SetupSignalHandler()
	schudulerController, err := controller.NewController(config, stopCh)
	if err != nil {
		klog.Fatalf("failed to start due to %v", err)
		return
	}
	go schudulerController.Run(threadness, stopCh)

	// set up kubernetes extender scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	predicate := server.NewElasticGPUPredicate(ctx, config)
	prioritize := server.NewElasticGPUPrioritize(ctx, config)
	bind := server.NewElasticGPUBind(ctx, config)

	// set up server
	router := httprouter.New()
	routes.AddPProf(router)
	routes.AddVersion(router)
	routes.AddPredicate(router, predicate)
	routes.AddPrioritize(router, prioritize)
	routes.AddBind(router, bind)

	klog.Infof("server starting on the port: %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		klog.Fatalf("failed to start server: %v", err)
	}
}

func StringToInt(sThread string) int {
	thread, err := strconv.Atoi(sThread)
	if err != nil || thread < 1 {
		return 1
	}

	return thread
}
