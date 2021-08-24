package dealer

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	schetypes "github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	log "k8s.io/klog/v2"
)

const OptimisticLockErrorMsg = "the object has been modified; please apply your changes to the latest version and try again"

type Dealer interface {
	Assume(nodes []string, pod *v1.Pod) ([]bool, []error)
	Score(node []string, pod *v1.Pod) []int
	Bind(node string, pod *v1.Pod) error
	Allocate(pod *v1.Pod) error
	Release(pod *v1.Pod) error
	KnownPod(pod *v1.Pod) bool
}

func NewDealer(clientset *kubernetes.Clientset, nodeLister corelisters.NodeLister, podLister corelisters.PodLister, rater Rater) (Dealer, error) {
	di := &DealerImpl{
		client:     clientset,
		nodeLister: nodeLister,
		podLister:  podLister,
		rater:      rater,
		lock:       sync.Mutex{},
		podMaps:    make(map[types.UID]*v1.Pod),
		nodeMaps:   make(map[string]*NodeInfo),
	}
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", schetypes.GPUAssume, "true"),
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

type DealerImpl struct {
	client     *kubernetes.Clientset
	nodeLister corelisters.NodeLister
	podLister  corelisters.PodLister

	rater    Rater
	lock     sync.Mutex
	podMaps  map[types.UID]*v1.Pod
	nodeMaps map[string]*NodeInfo
}

func (d *DealerImpl) Assume(nodes []string, pod *v1.Pod) ([]bool, []error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	demand := NewDemandFromPod(pod)
	res := make([]error, len(nodes))
	ans := make([]bool, len(nodes))
	nodeInfos := make([]*NodeInfo, len(nodes))
	for i, name := range nodes {
		ni, err := d.getNodeInfo(name)
		if err != nil {
			ni = nil
			ans[i] = false
			res[i] = fmt.Errorf("nano gpu scheduler get node failed: %s", err.Error())
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
					assumed, err := nodeInfos[number].Assume(demand)
					ans[number] = assumed
					res[number] = err
				default:
					return
				}
			}

		}()
	}
	wg.Wait()
	return ans, res
}

func (d *DealerImpl) Score(nodes []string, pod *v1.Pod) []int {
	d.lock.Lock()
	defer d.lock.Unlock()
	demand := NewDemandFromPod(pod)
	scores := make([]int, len(nodes))
	for i := 0; i < len(nodes); i++ {
		ni, err := d.getNodeInfo(nodes[i])
		if err != nil {
			log.Errorf("score pod %s/%s not found target node %s: %s", pod.Namespace, pod.Name, nodes[i], err.Error())
			scores[i] = ScoreMin
			continue
		}
		scores[i] = ni.Score(demand)
	}
	return scores
}

func (d *DealerImpl) Bind(node string, pod *v1.Pod) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	ni, err := d.getNodeInfo(node)
	if err != nil {
		return err
	}
	plan, err := ni.Bind(NewDemandFromPod(pod))
	if err != nil {
		return err
	}

	newPod := utils.GetUpdatedPodAnnotationSpec(pod, plan.GPUIndexes)
	if _, err := d.client.CoreV1().Pods(newPod.Namespace).Update(context.Background(), newPod, metav1.UpdateOptions{}); err != nil {
		if err.Error() == OptimisticLockErrorMsg {
			pod, err = d.client.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			newPod = utils.GetUpdatedPodAnnotationSpec(pod, plan.GPUIndexes)
			if _, err = d.client.CoreV1().Pods(pod.Namespace).Update(context.Background(), newPod, metav1.UpdateOptions{}); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
	if err := d.client.CoreV1().Pods(newPod.Namespace).Bind(context.Background(), &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{Namespace: newPod.Namespace, Name: newPod.Name, UID: newPod.UID},
		Target: v1.ObjectReference{
			Kind: "Node",
			Name: node,
		},
	}, metav1.CreateOptions{}); err != nil {
		return err
	}
	d.podMaps[pod.UID] = newPod
	d.status("bind")
	return nil
}

func (d *DealerImpl) Allocate(pod *v1.Pod) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if pod.Name == "" {
		return fmt.Errorf("pod %s/%s nodename is empty", pod.Namespace, pod.Name)
	}
	ni, err := d.getNodeInfo(pod.Spec.NodeName)
	if err != nil {
		return err
	}
	if _, ok := d.podMaps[pod.UID]; ok {
		return nil
	}
	plan, err := NewPlanFromPod(pod)
	if err != nil {
		return err
	}
	err = ni.Allocate(plan)
	if err != nil {
		return err
	}
	d.podMaps[pod.UID] = pod
	d.status("allocate")
	return nil
}

func (d *DealerImpl) Release(pod *v1.Pod) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	ni, err := d.getNodeInfo(pod.Spec.NodeName)
	if err != nil {
		log.Errorf("release pod %s failed: %s", pod.Name, err.Error())
		return err
	}
	if _, ok := d.podMaps[pod.UID]; !ok {
		log.Errorf("no such pod %s/%s", pod.Namespace, pod.Name)
		return nil
	}
	plan, err := NewPlanFromPod(pod)
	if err != nil {
		log.Errorf("create plan from pod failed: %s", err.Error())
		return err
	}
	if err := ni.Release(plan); err != nil {
		log.Errorf("release pod %s failed: node info release failed: %s", pod.Name, err.Error())
		return err
	}
	delete(d.podMaps, pod.UID)
	d.status("release")
	return nil
}

func (d *DealerImpl) KnownPod(pod *v1.Pod) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	_, ok := d.podMaps[pod.UID]
	return ok
}

func (d *DealerImpl) getNodeInfo(name string) (*NodeInfo, error) {
	if ni, ok := d.nodeMaps[name]; ok {
		return ni, nil
	}
	node, err := d.nodeLister.Get(name)
	if err != nil {
		return nil, err
	}
	pods, err := d.client.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", schetypes.GPUAssume, "true"),
		FieldSelector: fields.OneTermEqualSelector(schetypes.NodeNameField, name).String(),
	})
	if err != nil {
		return nil, err
	}
	d.nodeMaps[name] = NewNodeInfo(name, node, d.rater)
	for _, pod := range pods.Items {
		// todo: check pod status
		plan, err := NewPlanFromPod(&pod)
		if err != nil {
			log.Errorf("stat pod %s/%s failed: %s", pod.Namespace, pod.Name, err.Error())
			continue
		}
		if err := d.nodeMaps[name].Allocate(plan); err != nil {
			log.Errorf("allocate pod %s/%s failed: %s", pod.Namespace, pod.Name, err.Error())
			continue
		}
		d.podMaps[pod.UID] = &pod
	}
	return d.nodeMaps[name], nil
}

func (d *DealerImpl) status(action string) {
	log.Infof("-----%s-status-----", action)
	for name, node := range d.nodeMaps {
		log.Infof("node %s: %v\n", name, node.GPUs)
	}
}
