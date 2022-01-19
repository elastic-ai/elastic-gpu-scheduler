package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	DSCtx "github.com/nano-gpu/nano-gpu-scheduler/pkg/context"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/controller"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/routes"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/scheduler"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
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
	PolicyConfigPath  string
	DefaultPolicyConfigPath = "/data/policy.yaml"
	PrometheusUrl     string
	InstancePort      string
	SyncPeriod        time.Duration
	isLoadSchedule    bool

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
	flag.StringVar(&PriorityAlgorithm, "priority", "binpack", "priority algorithm, binpack/spread")
	flag.StringVar(&PolicyConfigPath, "policyConfigPath", DefaultPolicyConfigPath, "Policy Config Path")
	flag.StringVar(&PrometheusUrl, "prometheusUrl", "http://thanos-prometheus.kube-system:80",
		"The prometheus url, default: http://thanos-prometheus.kube-system:80.")
	flag.StringVar(&InstancePort, "instancePort",  "9100", "The instance port, default: 9100")
	flag.DurationVar(&SyncPeriod, "sync-period",  time.Second * 5, "sync period")
	flag.BoolVar(&isLoadSchedule, "isLoadSchedule",  false, "Is load scheduling enabled")


}

func main() {
	InitFlag()

	log.InitFlags(nil)
	flag.Parse()

	log.Info("Priority algorithm is ", PriorityAlgorithm)

	switch PriorityAlgorithm {
	case types.PrioritySpread:
		controller.Rater = &dealer.Spread{}
	case types.PriorityBinPack:
		controller.Rater = &dealer.Binpack{}
	default:
		log.Errorf("Priority algorithm %s is not supported", PriorityAlgorithm)
		return
	}

	threadness := StringToInt(os.Getenv("THREADNESS"))

	initKubeClient()
	port := os.Getenv("PORT")
	if _, err := strconv.Atoi(port); err != nil {
		port = "39999"
	}

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()
	informerFactory := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	schudulerController, err := controller.NewController(clientset, informerFactory, PrometheusUrl, InstancePort, PolicyConfigPath, SyncPeriod, isLoadSchedule, stopCh)
	if err != nil {
		log.Fatalf("Failed to start due to %v", err)
		return
	}

	go schudulerController.Run(threadness, stopCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var policy dealer.PolicySpec
	if isLoadSchedule {
		context := DSCtx.NewDSContext(PolicyConfigPath)
		context.Start()
		policy = context.GetPolicySpec()
	}

	predicate := scheduler.NewNanoGPUPredicate(ctx, clientset, schudulerController.GetDealer(), policy, isLoadSchedule)
	prioritize := scheduler.NewNanoGPUPrioritize(ctx, clientset, schudulerController.GetDealer(), policy, isLoadSchedule)
	bind := scheduler.NewNanoGPUBind(ctx, clientset, schudulerController.GetDealer(), policy, isLoadSchedule)

	router := httprouter.New()
	routes.AddPProf(router)
	routes.AddVersion(router)
	routes.AddPredicate(router, predicate)
	routes.AddPrioritize(router, prioritize)
	routes.AddBind(router, bind)
	routes.AddStatus(router, schudulerController.GetDealer())

	log.Infof("server starting on the port :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}

func StringToInt(sThread string) int {
	thread, err := strconv.Atoi(sThread)
	if err != nil || thread < 1 {
		return 1
	}

	return thread
}
