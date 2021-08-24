package cache

import (
	"sync"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	corelisters "k8s.io/client-go/listers/core/v1"
	log "k8s.io/klog/v2"
)

type SchedulerCache struct {
	// a map from pod key to podState.
	nodes map[string]*NodeInfo

	// record the knownPod, it will be added when annotation TENCENT_GPU_ID is added, and will be removed when complete and deleted
	knownPods map[types.UID]*v1.Pod

	nodeLister corelisters.NodeLister
	podLister  corelisters.PodLister
	nLock      *sync.RWMutex
}

func NewSchedulerCache(nLister corelisters.NodeLister, pLister corelisters.PodLister) *SchedulerCache {
	return &SchedulerCache{
		nodes:      make(map[string]*NodeInfo),
		knownPods:  make(map[types.UID]*v1.Pod),
		nodeLister: nLister,
		podLister:  pLister,
		nLock:      new(sync.RWMutex),
	}
}

// build cache when initializing
func (cache *SchedulerCache) BuildCache() error {
	log.Info("begin to build scheduler cache")
	pods, err := cache.podLister.List(labels.Everything())

	if err != nil {
		return err
	} else {
		for _, pod := range pods {
			if len(utils.GetGPUIDFromAnnotation(pod)) <= 0 {
				continue
			}

			if len(pod.Spec.NodeName) == 0 {
				continue
			}

			err = cache.AddOrUpdatePod(pod)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cache *SchedulerCache) GetPod(name, namespace string) (*v1.Pod, error) {
	return cache.podLister.Pods(namespace).Get(name)
}

// Get known pod from the pod UID
func (cache *SchedulerCache) KnownPod(podUID types.UID) bool {
	cache.nLock.RLock()
	defer cache.nLock.RUnlock()

	_, found := cache.knownPods[podUID]
	return found
}

func (cache *SchedulerCache) AddOrUpdatePod(pod *v1.Pod) error {
	log.V(5).Infof("Add or update pod %v", pod)
	log.V(5).Infof("Node %v", cache.nodes)
	if len(pod.Spec.NodeName) == 0 {
		log.Warningf("Pod %s in ns %s is not assigned to any node, skip", pod.Name, pod.Namespace)
		return nil
	}

	nodeInfo, err := cache.GetNodeInfo(pod.Spec.NodeName)
	if err != nil {
		return err
	}
	podCopy := pod.DeepCopy()
	if nodeInfo.addOrUpdatePod(podCopy) {
		// put it into known pod
		cache.rememberPod(pod.UID, podCopy)
	} else {
		log.Warningf("Pod %s in ns %s's gpu id is %v, it's illegal, skip", pod.Name, pod.Namespace, utils.GetGPUIDFromAnnotation(pod))
	}

	return nil
}

// The lock is in cacheNode
func (cache *SchedulerCache) RemovePod(pod *v1.Pod) {
	log.V(5).Infof("Remove pod %v", pod)
	log.V(5).Infof("Node %v", cache.nodes)
	n, err := cache.GetNodeInfo(pod.Spec.NodeName)
	if err == nil {
		n.removePod(pod)
	} else {
		log.Warningf("Failed to get node %s due to %v", pod.Spec.NodeName, err)
	}

	cache.forgetPod(pod.UID)
}

func (cache *SchedulerCache) forgetPod(uid types.UID) {
	cache.nLock.Lock()
	defer cache.nLock.Unlock()
	delete(cache.knownPods, uid)
}

func (cache *SchedulerCache) rememberPod(uid types.UID, pod *v1.Pod) {
	cache.nLock.Lock()
	defer cache.nLock.Unlock()
	cache.knownPods[pod.UID] = pod
}

func (cache *SchedulerCache) GetNodeInfos() []*NodeInfo {
	nodes := []*NodeInfo{}
	for _, n := range cache.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// Get or build nodeInfo if it doesn't exist
func (cache *SchedulerCache) GetNodeInfo(name string) (*NodeInfo, error) {
	node, err := cache.nodeLister.Get(name)
	if err != nil {
		return nil, err
	}

	cache.nLock.Lock()
	defer cache.nLock.Unlock()
	nodeInfo, ok := cache.nodes[name]

	if !ok {
		nodeInfo = NewNodeInfo(node)
		cache.nodes[name] = nodeInfo
	} else {
		// // if the existing node turn from non gpushare to gpushare
		// if (utils.GetTotalGPUPercent(n.node) <= 0 && utils.GetTotalGPUPercent(node) > 0) ||
		// 	// if the existing node turn from gpushare to non gpushare
		// 	(utils.GetTotalGPUPercent(n.node) > 0 && utils.GetTotalGPUPercent(node) <= 0)
		if len(nodeInfo.devs) == 0 || utils.GetGPUDeviceCountOfNode(nodeInfo.node) <= 0 {
			log.Infof("GetNodeInfo() need update node %s", name)
			// fix the scenario that the number of devices changes from 0 to an positive number
			nodeInfo.Reset(node)
			log.Infof("node: %s, labels from cache after been updated: %v", nodeInfo.node.Name, nodeInfo.node.Labels)
		} else {
			log.V(5).Infof("GetNodeInfo() uses the existing nodeInfo for %s", name)
		}
		log.V(5).Infof("node %s with devices %v", name, nodeInfo.devs)
	}
	return nodeInfo, nil
}
