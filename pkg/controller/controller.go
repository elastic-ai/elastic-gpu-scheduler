package controller

import (
	"fmt"
	"time"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	log "k8s.io/klog/v2"
)

var (
	KeyFunc = clientgocache.DeletionHandlingMetaNamespaceKeyFunc
)

var Rater dealer.Rater

type Controller struct {
	clientset *kubernetes.Clientset

	// podLister can list/get pods from the shared informer's store.
	podLister corelisters.PodLister

	// nodeLister can list/get nodes from the shared informer's store.
	nodeLister corelisters.NodeLister

	// podQueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	podQueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// podInformerSynced returns true if the pod store has been synced at least once.
	podInformerSynced clientgocache.InformerSynced

	// nodeInformerSynced returns true if the service store has been synced at least once.
	nodeInformerSynced clientgocache.InformerSynced

	dealer dealer.Dealer
}

func NewController(clientset *kubernetes.Clientset, kubeInformerFactory informers.SharedInformerFactory, stopCh <-chan struct{}) (c *Controller, err error) {
	log.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "nano-gpu-scheduler"})

	c = &Controller{
		clientset: clientset,
		podQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podQueue"),
		recorder:  recorder,
	}
	// Create pod informer.
	podInformer := kubeInformerFactory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(clientgocache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *v1.Pod:
				return utils.IsGPUSharingPod(t)
			case clientgocache.DeletedFinalStateUnknown:
				if pod, ok := t.Obj.(*v1.Pod); ok {
					log.Infof("delete pod %s/%s", pod.Namespace, pod.Name)
					return utils.IsGPUSharingPod(pod)
				}
				runtime.HandleError(fmt.Errorf("unable to convert object %T to *v1.Pod in %T", obj, c))
				return false
			default:
				runtime.HandleError(fmt.Errorf("unable to handle object in %T: %T", c, obj))
				return false
			}
		},
		Handler: clientgocache.ResourceEventHandlerFuncs{
			AddFunc:    c.addPodToCache,
			UpdateFunc: c.updatePodInCache,
			DeleteFunc: c.deletePodFromCache,
		},
	})

	c.podLister = podInformer.Lister()
	c.podInformerSynced = podInformer.Informer().HasSynced

	// Create node informer
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	c.nodeLister = nodeInformer.Lister()
	c.nodeInformerSynced = nodeInformer.Informer().HasSynced

	// Start informer goroutines.
	go kubeInformerFactory.Start(stopCh)

	// Create scheduler Cache
	c.dealer, err = dealer.NewDealer(c.clientset, c.nodeLister, c.podLister, Rater)
	if err != nil {
		log.Error("create dealer failed: %s", err.Error())
		return nil, err
	}

	log.Info("begin to wait for cache")

	if ok := clientgocache.WaitForCacheSync(stopCh, c.nodeInformerSynced); !ok {
		return nil, fmt.Errorf("failed to wait for node caches to sync")
	} else {
		log.Info("init the node cache successfully")
	}

	if ok := clientgocache.WaitForCacheSync(stopCh, c.podInformerSynced); !ok {
		return nil, fmt.Errorf("failed to wait for pod caches to sync")
	} else {
		log.Info("init the pod cache successfully")
	}

	log.Info("end to wait for cache")

	return c, nil
}

func (c *Controller) GetDealer() dealer.Dealer {
	return c.dealer
}

// Run will set up the event handlers
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.podQueue.ShutDown()

	log.Info("Starting GPU Sharing Controller.")
	log.Info("Waiting for informer caches to sync")

	log.Infof("Starting %v workers.", threadiness)
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// syncPod will sync the pod with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its pods created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (c *Controller) syncPod(key string) (forget bool, err error) {
	log.V(2).Infof("begin to sync controller for pod %s", key)
	ns, name, err := clientgocache.SplitMetaNamespaceKey(key)
	if err != nil {
		return false, err
	}

	pod, err := c.podLister.Pods(ns).Get(name)
	switch {
	case errors.IsNotFound(err):
		log.V(2).Infof("pod %s/%s has been deleted.", ns, name)
	case err != nil:
		log.Warningf("unable to retrieve pod %v from the store: %v", key, err)
	default:
		if utils.IsCompletedPod(pod) {
			log.V(2).Infof("pod %s/%s has completed.", ns, name)
			if err := c.dealer.Release(pod); err != nil {
				log.Errorf("release pod %s/%s failed: %s", pod.Namespace, pod.Name, err.Error())
			}
			c.dealer.PrintStatus(pod, "release")
		} else {
			if pod.Spec.NodeName == "" {
				return true, nil
			}
			err := c.dealer.Allocate(pod)
			c.dealer.PrintStatus(pod, "allocate")
			if err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

// processNextWorkItem will read a single work item off the podQueue and
// attempt to process it.
func (c *Controller) processNextWorkItem() bool {
	log.V(4).Info("begin processNextWorkItem()")
	key, quit := c.podQueue.Get()
	if quit {
		return false
	}
	defer c.podQueue.Done(key)
	defer log.V(4).Info("end processNextWorkItem()")
	forget, err := c.syncPod(key.(string))
	if err == nil {
		if forget {
			c.podQueue.Forget(key)
		}
		return false
	}

	log.Infof("Error syncing pods: %v", err)
	runtime.HandleError(fmt.Errorf("Error syncing pod: %v", err))
	c.podQueue.AddRateLimited(key)

	return true
}

func (c *Controller) addPodToCache(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Warningf("cannot convert to *v1.Pod: %v", obj)
		return
	}

	podKey, err := KeyFunc(pod)
	if err != nil {
		log.Warningf("Failed to get the jobkey: %v", err)
		return
	}

	c.podQueue.Add(podKey)

	// NOTE: Updating equivalence cache of addPodToCache has been
	// handled optimistically in: pkg/scheduler/scheduler.go#assume()
}

func (c *Controller) updatePodInCache(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		log.Warningf("cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		log.Warningf("cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}
	needUpdate := false

	// 1. Need update when pod is turned to complete or failed
	if c.dealer.KnownPod(oldPod) && utils.IsCompletedPod(newPod) {
		needUpdate = true
	}
	// 2. Need update when it's unknown and unreleased pod, and GPU annotation has been set
	if !c.dealer.KnownPod(oldPod) && !c.dealer.PodReleased(oldPod) && utils.IsAssumed(newPod) {
		needUpdate = true
	}
	if needUpdate {
		podKey, err := KeyFunc(newPod)
		if err != nil {
			log.Warningf("Failed to get the job key: %v", err)
			return
		}
		log.V(2).Infof("Need to update pod name %s/%s and old status is %v, new status is %v; its old annotation %v and new annotation %v",
			newPod.Namespace,
			newPod.Name,
			oldPod.Status.Phase,
			newPod.Status.Phase,
			oldPod.Annotations,
			newPod.Annotations)
		c.podQueue.Add(podKey)
	} else {
		log.V(4).Infof("No need to update pod name %s/%s and old status is %v, new status is %v; its old annotation %v and new annotation %v",
			newPod.Namespace,
			newPod.Name,
			oldPod.Status.Phase,
			newPod.Status.Phase,
			oldPod.Annotations,
			newPod.Annotations)
	}

	return
}

func (c *Controller) deletePodFromCache(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case clientgocache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			log.Warning("cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		log.Warning("cannot convert to *v1.Pod: %v", t)
		return
	}

	log.Infof("delete pod %s/%s", pod.Namespace, pod.Name)

	c.dealer.Forget(pod)
}
