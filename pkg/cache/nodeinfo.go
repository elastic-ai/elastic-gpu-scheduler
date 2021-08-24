package cache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"

	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog/v2"
)

const (
	OptimisticLockErrorMsg = "the object has been modified; please apply your changes to the latest version and try again"
)

var PriorityAlgorithm string

// TODO 1. what if new node is found ?
// TODO 2. context
// TODO 3. available gpu computing
// TODO 4. gpu health check
// TODO 5. another 2 priority algorithm

// NodeInfo is node level aggregated information.
type NodeInfo struct {
	name            string
	node            *v1.Node
	devs            map[int]*DeviceInfo
	candidateDevIds []int
	score           int
	gpuCount        int
	gpuTotalMemory  uint
	deviceTotalMemory uint
	rwmu            *sync.RWMutex
}

func NewNodeInfo(node *v1.Node) *NodeInfo {
	devMap := map[int]*DeviceInfo{}
	var deviceTotalMemory uint

	nodeTotalMemory := uint(utils.GetTotalGPUMemory(node))
	gpuCount := utils.GetGPUDeviceCountOfNode(node)
	if gpuCount > 0 {
		deviceTotalMemory = nodeTotalMemory / uint(gpuCount)
	}

	for i := 0; i < gpuCount; i++ {
		devMap[i] = newDeviceInfo(i, deviceTotalMemory)
	}

	log.V(5).Infof("NewNodeInfo() creates nodeInfo for %s: device map %v", node.Name, devMap)

	if len(devMap) == 0 {
		log.Warningf("node %s with nodeinfo has no gpu devices", node.Name)
	}

	return &NodeInfo{
		name:            node.Name,
		node:            node,
		devs:            devMap,
		candidateDevIds: []int{},
		score:           0,
		gpuCount:        gpuCount,
		gpuTotalMemory:  nodeTotalMemory,
		deviceTotalMemory: deviceTotalMemory,
		rwmu:            new(sync.RWMutex),
	}
}

// Only update the devices when the length of devs is 0
func (n *NodeInfo) Reset(node *v1.Node) {
	n.gpuTotalMemory = uint(utils.GetTotalGPUMemory(node))
	n.gpuCount = utils.GetGPUDeviceCountOfNode(node)
	if n.gpuCount == 0 {
		log.Warningf("Reset for node %s but the gpu count is 0", node.Name)
	}

	if n.gpuTotalMemory == 0 {
		log.Warningf("Reset for node %s but the gpu total memory is 0", node.Name)
	}

	if len(n.devs) == 0 && n.gpuCount > 0 {
		devMap := map[int]*DeviceInfo{}
		deviceTotalMemory := n.gpuTotalMemory / uint(n.gpuCount)
		for i := 0; i < n.gpuCount; i++ {
			devMap[i] = newDeviceInfo(i, deviceTotalMemory)
		}
		n.devs = devMap
		n.deviceTotalMemory = deviceTotalMemory
	}

	log.Infof("Reset() update nodeInfo for %s with devs %v", node.Name, n.devs)
}

func (n *NodeInfo) GetName() string {
	return n.name
}

func (n *NodeInfo) GetDevs() []*DeviceInfo {
	devs := make([]*DeviceInfo, n.gpuCount)
	for i, dev := range n.devs {
		devs[i] = dev
	}
	return devs
}

func (n *NodeInfo) GetNode() *v1.Node {
	return n.node
}

func (n *NodeInfo) GetTotalGPUMemory() uint {
	return n.gpuTotalMemory
}

func (n *NodeInfo) GetGPUCount() int {
	return n.gpuCount
}

func (n *NodeInfo) removePod(pod *v1.Pod) {
	n.rwmu.Lock()
	defer n.rwmu.Unlock()

	// len(gpuIds) would only be 1
	gpuIds := utils.GetGPUIDFromAnnotation(pod)
	gpuCore := utils.GetGPUCoreFromPodResource(pod)
	gpuMemory := utils.GetGPUMemoryFromPodResource(pod)
	if len(gpuIds) > 0 {
		for _, id := range gpuIds {
			dev, found := n.devs[id]
			if !found {
				log.Warningf("Pod %s in ns %s failed to find the GPU ID %d in node %s", pod.Name, pod.Namespace, id, n.name)
			} else {
				dev.RemovePod(pod.Name, gpuCore, gpuMemory)
			}
		}
	} else {
		log.Warningf("Pod %s in ns %s is not set the GPU ID %v in node %s", pod.Name, pod.Namespace, gpuIds, n.name)
	}
}

// Add the Pod which has the GPU id to the node
func (n *NodeInfo) addOrUpdatePod(pod *v1.Pod) (added bool) {
	n.rwmu.Lock()
	defer n.rwmu.Unlock()

	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		log.Warningf("skip the pod %s in ns %s due to its status is %s", pod.Name, pod.Namespace, pod.Status.Phase)
		return false
	}

	// len(gpuIds) would only be 1
	gpuIds := utils.GetGPUIDFromAnnotation(pod)
	gpuCore := utils.GetGPUCoreFromPodResource(pod)
	gpuMemory := utils.GetGPUMemoryFromPodResource(pod)
	if len(gpuIds) > 0 {
		for _, id := range gpuIds {
			dev, found := n.devs[id]
			if !found {
				log.Warningf("Pod %s in ns %s failed to find the GPU ID %d in node %s", pod.Name, pod.Namespace, id, n.name)
			} else {
				dev.AddPod(pod.Name, gpuCore, gpuMemory)
				added = true
			}
		}
	} else {
		log.Warningf("Pod %s in ns %s is not set the GPU ID %v in node %s", pod.Name, pod.Namespace, gpuIds, n.name)
	}
	return added
}

func (n *NodeInfo) Assume(pod *v1.Pod) (allocatable bool) {
	allocatable = false

	n.rwmu.RLock()
	defer n.rwmu.RUnlock()

	// reset candidateDevIds and score for each pod we scheduled
	n.candidateDevIds = []int{}
	n.score = 0

	gpuCore := utils.GetGPUCoreFromPodResource(pod)
	gpuMemory := utils.GetGPUMemoryFromPodResource(pod)

	if gpuCore >= types.GPUCoreEachCard {
		// gpuCore >= 100: exclusive mode, not support yet
		log.Warningf("Not GPU required pod, request gpu core %d", gpuCore)
		return false
	}

	availGPUCore, availGPUMemory := n.getAvailableGPUs()
	log.Infof("AvailableGPUs: %v in node %s", availGPUCore, n.name)
	if len(availGPUCore) < 0 {
		return false
	}

	n.candidateDevIds, n.score = n.ScoreCard(availGPUCore, availGPUMemory, gpuCore, gpuMemory)

	// if reqGPU < 100: sharing mode
	if len(n.candidateDevIds) > 0 {
		allocatable = true
	}

	return allocatable
}

func (n *NodeInfo) ScoreCard(availGPUCore, availGPUMemory map[int]uint, gpuCore, gpuMemory uint) (candidateDevIds []int, score int) {
	switch PriorityAlgorithm {
	case types.PrioritySpread:
		candidateDevIds, score = n.Spread(availGPUCore, availGPUMemory, gpuCore, gpuMemory)
	case types.PriorityRandom:
		candidateDevIds, score = n.Random(availGPUCore, availGPUMemory, gpuCore, gpuMemory)
	case types.PriorityBinPack:
		candidateDevIds, score = n.BinPack(availGPUCore, availGPUMemory, gpuCore, gpuMemory)
	default:
		log.Warningf("Priority algorithm %s is not supported", PriorityAlgorithm)
	}
	return candidateDevIds, score
}

func (n *NodeInfo) BinPack(availGPUCores, availGPUMemorys map[int]uint, gpuCore, gpuMemory uint) (candidateDevIds []int, score int) {
	if len(availGPUCores) < 0 {
		return
	}

	if gpuCore >= types.GPUCoreEachCard {
		// if gpuCore >= 100: exclusive mode
		log.Fatalf("Not GPU required pod, request gpu core %d", gpuCore)
		return candidateDevIds, score
	} else {
		// if reqGPU < 100: sharing mode
		// binpack: choose the node which has least available gpu core and least available gpu memory
		// we would loop over the gpus on the nodes, compute score for each card, choose the maximum score of the cards
		// consider we have pass filter phase, so the available gpu percent is more then request gpu percent
		scoreByDevice := 0
		for devID := 0; devID < len(n.devs); devID++ {
			availGPUCore, ok := availGPUCores[devID]
			availGPUMem, ok := availGPUMemorys[devID]
			if ok {
				if availGPUCore < gpuCore || availGPUMem < gpuMemory{
					continue
				}
				// scoreByDeviceCore = { 100 - (availableGPU - reqGPU) } * 10 / 100
				scoreByDeviceCore := int((types.GPUCoreEachCard - availGPUCore + gpuCore) * 10 / types.GPUCoreEachCard)
				scoreByDeviceMem := int((n.deviceTotalMemory - availGPUMem + gpuMemory) * 10 / n.deviceTotalMemory)
				scoreByDevice = (scoreByDeviceCore + scoreByDeviceMem) / 2
				if scoreByDevice > score {
					score = scoreByDevice
					candidateDevIds = []int{devID}
				} else if scoreByDevice == score {
					candidateDevIds = append(n.candidateDevIds, devID)
				}
			}
		}
	}
	return candidateDevIds, score
}

func (n *NodeInfo) Spread(availGPUCores, availGPUMemorys map[int]uint, gpuCore, gpuMemory uint) (candidateDevIds []int, score int) {
	if len(availGPUCores) < 0 {
		return
	}

	if gpuCore >= types.GPUCoreEachCard {
		// if gpuCore >= 100: exclusive mode
		log.Fatalf("Not GPU required pod, request gpu core %d", gpuCore)
		return candidateDevIds, score
	} else {
		// if reqGPU < 100: sharing mode
		// spread: choose the node which has most available percent card
		// we would loop over the gpus on the nodes, compute score for each card, choose the maximum score of the cards
		// consider we have pass filter phase, so the available gpu percent is more then request gpu percent
		scoreByDevice := 0
		for devID := 0; devID < len(n.devs); devID++ {
			availGPUCore, ok := availGPUCores[devID]
			availGPUMem, ok := availGPUMemorys[devID]
			if ok {
				if availGPUCore < gpuCore || availGPUMem < gpuMemory{
					continue
				}
				// scoreByDevice = availableGPU * 10 / 100
				// if availableGPU == 100 -> score = 10
				// if (availableGPU == reqGPU) {we already have} -> score = availableGPU / 10
				scoreByDeviceCore := int(availGPUCore * 10 / types.GPUCoreEachCard)
				scoreByDeviceMem := int(availGPUMem * 10 / n.deviceTotalMemory)
				scoreByDevice = (scoreByDeviceCore + scoreByDeviceMem) / 2
				if scoreByDevice > score {
					score = scoreByDevice
					candidateDevIds = []int{devID}
				} else if scoreByDevice == score {
					candidateDevIds = append(n.candidateDevIds, devID)
				}
			}
		}
	}
	return candidateDevIds, score
}

func (n *NodeInfo) Random(availGPUCores, availGPUMemorys map[int]uint, gpuCore, gpuMemory uint) (candidateDevIds []int, score int) {
	if len(availGPUCores) < 0 {
		return
	}

	rand.Seed(time.Now().UnixNano())

	if gpuCore >= types.GPUCoreEachCard {
		// if gpuCore >= 100: exclusive mode
		log.Fatalf("Not GPU required pod, request gpu core %d", gpuCore)
		return candidateDevIds, score
	} else {
		// if reqGPU < 100: sharing mode
		// random: choose the node randomly which satisfies the request gpu percent
		// we would loop over the gpus on the node, compute score for each card, choose the maximum score of the cards
		// consider we have pass filter phase, so the available gpu percent is more then request gpu percent
		scoreByDevice := 0
		for devID := 0; devID < len(n.devs); devID++ {
			availGPUCore, ok := availGPUCores[devID]
			availGPUMem, ok := availGPUMemorys[devID]
			if ok {
				if availGPUCore < gpuCore || availGPUMem < gpuMemory{
					continue
				}
				// scoreByDevice = rand.Intn(10)
				scoreByDevice = rand.Intn(10)
				if scoreByDevice > score {
					score = scoreByDevice
					candidateDevIds = []int{devID}
				} else if scoreByDevice == score {
					candidateDevIds = append(n.candidateDevIds, devID)
				}
			}
		}
	}
	return candidateDevIds, score
}

// Score
func (n *NodeInfo) Score(pod *v1.Pod) (score int) {
	return n.score
}

// Allocate allocate gpu for the pod at binding period, write the gpu id to pod annotation
// since kube-scheduler choose which node to bind with the pod (considering the strategy of gpu scheduler extender)
// we now need to decide which gpu cards on the node for the pod
// here we also have 2 mode: gpu exclusive mode / sharing mode
func (n *NodeInfo) Allocate(ctx context.Context, clientset *kubernetes.Clientset, pod *v1.Pod) (err error) {
	var newPod *v1.Pod
	n.rwmu.Lock()
	defer n.rwmu.Unlock()
	log.Infof("Allocate GPU for gpu core/memory for pod %s in ns %s", pod.Name, pod.Namespace)
	// 1. Update the pod spec
	// devIds is an device id slice
	// when slice size == 1 -> sharing mode
	// when slice size > 1 -> exclusive mode
	gpuCore := utils.GetGPUCoreFromPodResource(pod)
	gpuMemory := utils.GetGPUMemoryFromPodResource(pod)
	devIds, found := n.allocateGPUID(pod)
	if found {
		log.Infof("Allocate GPU ID %v to pod %s in ns %s", devIds, pod.Name, pod.Namespace)
		newPod = utils.GetUpdatedPodAnnotationSpec(pod, devIds)
		_, err = clientset.CoreV1().Pods(newPod.Namespace).Update(ctx, newPod, metav1.UpdateOptions{})
		if err != nil {
			// the object has been modified; please apply your changes to the latest version and try again
			if err.Error() == OptimisticLockErrorMsg {
				// retry
				pod, err = clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				newPod = utils.GetUpdatedPodAnnotationSpec(pod, devIds)
				_, err = clientset.CoreV1().Pods(newPod.Namespace).Update(ctx, newPod, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
	} else {
		err = fmt.Errorf("the node %s can't place the pod %s in ns %s", pod.Spec.NodeName, pod.Name, pod.Namespace)
	}

	// 2. Bind the pod to the node
	if err == nil {
		binding := &v1.Binding{
			ObjectMeta: metav1.ObjectMeta{Name: pod.Name, UID: pod.UID},
			Target:     v1.ObjectReference{Kind: "Node", Name: n.name},
		}
		log.Infof("Bind pod %s in %s namespace to node %s", pod.Name, pod.Namespace, pod.Spec.NodeName)
		err = clientset.CoreV1().Pods(pod.Namespace).Bind(ctx, binding, metav1.CreateOptions{})
		if err != nil {
			log.Warningf("Failed to bind the pod %s in ns %s due to %v", pod.Name, pod.Namespace, err)
			return err
		}
	}

	// 3. update the device info if the pod is update successfully
	if err == nil {
		for _, devId := range devIds {
			log.Infof("Add pod %s in ns %s to device %d", pod.Name, pod.Namespace, devId)
			dev, found := n.devs[devId]
			if !found {
				log.Warningf("Pod %s in ns %s failed to find the GPU ID %d in node %s", pod.Name, pod.Namespace, devId, n.name)
			} else {
				dev.AddPod(pod.Name, gpuCore, gpuMemory)
			}
		}
	}
	log.Infof("Finish allocate gpu percent for pod %s in ns %s", pod.Name, pod.Namespace)
	return err
}

// allocate the GPU ID to the pod
func (n *NodeInfo) allocateGPUID(pod *v1.Pod) (candidateDevIDs []int, found bool) {
	found = false
	candidateDevIDs = []int{}

	availGPUCore, availGPUMemory := n.getAvailableGPUs()

	gpuCore := utils.GetGPUCoreFromPodResource(pod)
	gpuMemory := utils.GetGPUMemoryFromPodResource(pod)

	if gpuCore <= uint(0) {
		log.Warningf("Not GPU required pod, request gpu core %d", gpuCore)
		return candidateDevIDs, found
	}

	if gpuCore >= types.GPUCoreEachCard {
		// gpuCore >= 100: exclusive mode, not support yet
		log.Warningf("Not GPU required pod, request gpu core %d", gpuCore)
		return candidateDevIDs, found
	}

	// 0 < gpuCore < 100: sharing mode, isolated
	candidateDevID := -1
	log.Infof("totalReqGPU for pod %s in ns %s: gpu core %d, gpu memory %d", pod.Name, pod.Namespace, gpuCore, gpuMemory)
	log.Infof("availGPUCore: %v, availGPUMemory: %v in node %s", availGPUCore, availGPUMemory, n.name)

	if len(n.candidateDevIds) > 0 {
		found = true
		candidateDevID = n.candidateDevIds[0]
		candidateDevIDs = append(candidateDevIDs, candidateDevID)
	}

	if found {
		log.Infof("Find candidate dev id %d for pod %s in ns %s successfully.",
			candidateDevID,
			pod.Name,
			pod.Namespace)
	} else {
		log.Warningf("Failed to find available GPU Core %d, GPU Memory %d for the pod %s in the namespace %s",
			gpuCore, gpuMemory, pod.Name, pod.Namespace)
	}

	return candidateDevIDs, found
}

func (n *NodeInfo) getAvailableGPUs() (availGPUCore, availGPUMemory map[int]uint) {
	availGPUCore = make(map[int]uint)
	availGPUMemory = make(map[int]uint)

	for _, dev := range n.devs {
		availGPUCore[dev.idx] = dev.availGPUCore
		availGPUMemory[dev.idx] = dev.availGPUMemory
	}

	return availGPUCore, availGPUMemory
}
