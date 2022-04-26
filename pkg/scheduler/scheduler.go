package scheduler

import (
	"context"
	"elasticgpu.io/elastic-gpu/api/v1alpha1"
	"elasticgpu.io/elastic-gpu/clientset/versioned"
	"encoding/json"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	schetypes "elasticgpu.io/elastic-gpu-scheduler/pkg/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "k8s.io/klog/v2"
)

type ElasticSchedulerConfig struct {
	Clientset            *kubernetes.Clientset
	EGPUClientset        *versioned.Clientset
	RegisteredSchedulers map[v1.ResourceName]ResourceScheduler
	Rater                Rater
}

type ResourceScheduler interface {
	Assume(nodes []string, pod *v1.Pod) ([]string, map[string]string, error)
	Score(node []string, pod *v1.Pod) []int
	Bind(node string, pod *v1.Pod) error
	AddPod(pod *v1.Pod) error
	ForgetPod(pod *v1.Pod) error
	KnownPod(pod *v1.Pod) bool
	ReleasedPod(pod *v1.Pod) bool
	Status() string
}

type BaseScheduler struct {
	ElasticSchedulerConfig
	rater          Rater
	lock           sync.Mutex
	coreName       v1.ResourceName
	memName        v1.ResourceName
	podMaps        map[types.UID]*v1.Pod
	nodeMaps       map[string]*NodeAllocator
	releasedPodMap map[types.UID]struct{}
}

func newBaseScheduler(config ElasticSchedulerConfig, coreName v1.ResourceName, memName v1.ResourceName) BaseScheduler {
	return BaseScheduler{ElasticSchedulerConfig: config,
		rater:          config.Rater,
		coreName:       coreName,
		memName:        memName,
		podMaps:        make(map[types.UID]*v1.Pod),
		nodeMaps:       make(map[string]*NodeAllocator),
		releasedPodMap: make(map[types.UID]struct{})}
}

func (d *BaseScheduler) getNodeInfo(name string) (*NodeAllocator, error) {
	if na, ok := d.nodeMaps[name]; ok {
		return na, nil
	}
	node, err := d.Clientset.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	pods, err := d.Clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", schetypes.EGPUAssumed, "true"),
		FieldSelector: fields.OneTermEqualSelector(schetypes.NodeNameField, name).String(),
	})
	if err != nil {
		return nil, err
	}
	na, err := NewNodeAllocator(pods.Items, node, d.coreName, d.memName, d.rater)
	if err != nil {
		return nil, err
	}

	d.nodeMaps[name] = na
	return na, nil
}

func NewGPUUnitScheduler(config ElasticSchedulerConfig, coreName v1.ResourceName, memName v1.ResourceName) (ResourceScheduler, error) {
	di := &GPUUnitScheduler{
		BaseScheduler: newBaseScheduler(config, coreName, memName),
	}
	pods, err := di.Clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", schetypes.EGPUAssumed, "true"),
	})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" {
			continue
		}
		if _, err := di.getNodeInfo(pod.Spec.NodeName); err != nil {
			log.Errorf("get node %s failed: %s", pod.Spec.NodeName, err.Error())
			continue
		}
	}
	return di, nil
}

type GPUUnitScheduler struct {
	BaseScheduler
}

func (d *GPUUnitScheduler) Assume(nodes []string, pod *v1.Pod) ([]string, map[string]string, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	res := make([]error, len(nodes))
	ans := make([]bool, len(nodes))
	nodeInfos := make([]*NodeAllocator, len(nodes))
	for i, name := range nodes {
		ni, err := d.getNodeInfo(name)
		if err != nil {
			ni = nil
			ans[i] = false
			res[i] = fmt.Errorf("elastic gpu scheduler get node failed: %v", err)
		}
		nodeInfos[i] = ni
	}

	ch := make(chan int, len(nodeInfos))
	wg := sync.WaitGroup{}
	for i := 0; i < len(nodeInfos); i++ {
		ch <- i
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case number := <-ch:
					if nodeInfos[number] == nil {
						continue
					}
					ids, err := nodeInfos[number].Assume(pod)
					klog.Infof("assume: %s %v, err: %v", nodes[number], ids, err)
					ans[number] = ids != nil
					res[number] = err
				default:
					return
				}
			}

		}()
	}
	wg.Wait()

	filterdNodes := []string{}
	failedNodes := map[string]string{}
	for i := 0; i < len(ans); i++ {
		if ans[i] {
			filterdNodes = append(filterdNodes, nodes[i])
		} else {
			failedNodes[nodes[i]] = res[i].Error()
		}
	}
	// TODO: need remove
	for _, n := range d.nodeMaps {
		s, _ := json.Marshal(n.allocated)
		klog.Infof("node allcated: %s", string(s))
	}
	return filterdNodes, failedNodes, nil
}

func (d *GPUUnitScheduler) Score(nodes []string, pod *v1.Pod) []int {
	d.lock.Lock()
	defer d.lock.Unlock()
	scores := make([]int, len(nodes))
	for i := 0; i < len(nodes); i++ {
		ni, err := d.getNodeInfo(nodes[i])
		if err != nil {
			log.Errorf("score pod %s/%s not found target node %s: %s", pod.Namespace, pod.Name, nodes[i], err.Error())
			scores[i] = ScoreMin
			continue
		}
		scores[i] = ni.Score(pod)
	}
	return scores
}

func (d *GPUUnitScheduler) Bind(node string, pod *v1.Pod) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	ni, err := d.getNodeInfo(node)
	if err != nil {
		return err
	}
	ids, err := ni.Allocate(pod)
	if err != nil {
		return err
	}

	newPod := GetUpdatedPodAnnotationSpec(pod, ids)
	if _, err := d.Clientset.CoreV1().Pods(newPod.Namespace).Update(context.Background(), newPod, metav1.UpdateOptions{}); err != nil {
		if err.Error() == schetypes.OptimisticLockErrorMsg {
			pod, err = d.Clientset.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			newPod = GetUpdatedPodAnnotationSpec(pod, ids)
			if _, err = d.Clientset.CoreV1().Pods(pod.Namespace).Update(context.Background(), newPod, metav1.UpdateOptions{}); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
	if err := d.Clientset.CoreV1().Pods(newPod.Namespace).Bind(context.Background(), &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{Namespace: newPod.Namespace, Name: newPod.Name, UID: newPod.UID},
		Target: v1.ObjectReference{
			Kind: "Node",
			Name: node,
		},
	}, metav1.CreateOptions{}); err != nil {
		return err
	}
	d.podMaps[pod.UID] = newPod

	return nil
}

func (d *GPUUnitScheduler) AddPod(pod *v1.Pod) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if pod.Spec.NodeName == "" {
		return fmt.Errorf("pod %s/%s nodename is empty", pod.Namespace, pod.Name)
	}
	ni, err := d.getNodeInfo(pod.Spec.NodeName)
	if err != nil {
		return err
	}
	if _, ok := d.podMaps[pod.UID]; ok {
		return nil
	}
	ni.Add(pod, nil)
	d.podMaps[pod.UID] = pod
	return nil
}

func (d *GPUUnitScheduler) ForgetPod(pod *v1.Pod) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if pod.Spec.NodeName != "" {
		ni, err := d.getNodeInfo(pod.Spec.NodeName)
		if err != nil {
			return err
		}
		if err := ni.Forget(pod); err != nil {
			return err
		}
	}
	if _, ok := d.podMaps[pod.UID]; ok {
		delete(d.podMaps, pod.UID)
		d.releasedPodMap[pod.UID] = struct{}{}
	}

	return nil
}

func (d *GPUUnitScheduler) KnownPod(pod *v1.Pod) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	_, ok := d.podMaps[pod.UID]
	return ok
}

func (d *GPUUnitScheduler) ReleasedPod(pod *v1.Pod) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	_, ok := d.releasedPodMap[pod.UID]
	return ok
}

func (d *GPUUnitScheduler) Status() string {
	gpus := make(map[string]GPUs)
	for k, v := range d.nodeMaps {
		gpus[k] = v.GPUs
	}
	result, _ := json.Marshal(gpus)
	return string(result)
}

func BuildResourceSchedulers(modes []string, config ElasticSchedulerConfig) (map[v1.ResourceName]ResourceScheduler, error) {
	sches := map[v1.ResourceName]ResourceScheduler{}
	for _, m := range modes {
		switch m {
		case "pgpu":
			// TODO:
			//d, err := NewPGPUScheduler(config)
			//if err != nil {
			//	return nil, err
			//}
			//sches[v1alpha1.ResourcePGPU] = d
		case "gpushare":
			d, err := NewGPUUnitScheduler(config, v1alpha1.ResourceGPUCore, v1alpha1.ResourceGPUMemory)
			if err != nil {
				return nil, err
			}
			sches[v1alpha1.ResourceGPUCore] = d
			sches[v1alpha1.ResourceGPUMemory] = d
			//case "qgpu":
			//	d, err := NewGPUUnitScheduler(config, v1alpha1.ResourceQGPUCore, v1alpha1.ResourceQGPUMemory)
			//	if err != nil {
			//		return nil, err
			//	}
			//	sches[v1alpha1.ResourceQGPUCore] = d
			//	sches[v1alpha1.ResourceQGPUMemory] = d
		}
	}

	return sches, nil
}

func GetResourceScheduler(pod *v1.Pod, registeredSchedulers map[v1.ResourceName]ResourceScheduler) (ResourceScheduler, error) {
	for _, c := range pod.Spec.Containers {
		for k, _ := range c.Resources.Requests {
			d := registeredSchedulers[k]
			if d != nil {
				return d, nil
			}
		}
	}

	return nil, fmt.Errorf("cannot find scheduler for pod: %v", pod)
}
