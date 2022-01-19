package controller

import (
	"fmt"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"
	"k8s.io/apimachinery/pkg/labels"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog"
)

const (
	maxRetries                         = 5
	timeFormat                         = "2006-01-02T15:04:05Z"
	zone                               = "Asia/Shanghai"
	ResourceGPUPercent v1.ResourceName = "nano-gpu/gpu-percent"
	GPUPercentEachCard                 = 100
)

func (c *Controller) nodeWorker() {
	for c.processNodeWorkItem() {
	}
}

func (c *Controller) syncMetricLoop(metrics string, period time.Duration) {
	c.syncMetric(metrics)

	//begin loop
	tick := time.NewTicker(period)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			c.syncMetric(metrics)
		}
	}
}

func (c *Controller) syncMetric(metrics string) {
	nodeList, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Unable to list nodes (%v);", err)
		panic(fmt.Errorf("Unable to list nodes (%v);", err))
	}

	for _, node := range nodeList {
		c.nodeQueue.Add(node.Name + "/" + metrics)
	}
}

func (c *Controller) processNodeWorkItem() bool {
	key, quit := c.nodeQueue.Get()
	if quit {
		return false
	}
	defer c.nodeQueue.Done(key)
	err := c.syncNode(key.(string))
	c.handleSyncNodeErr(err, key)
	return true
}

func (c *Controller) handleSyncNodeErr(err error, key interface{}) {
	if err == nil {
		c.nodeQueue.Forget(key)
		return
	}

	if c.nodeQueue.NumRequeues(key) < maxRetries {
		klog.V(2).Infof("Error syncing SyncNode for id %s, retrying. Error: %v", key, err)
		c.nodeQueue.AddRateLimited(key)
		return
	}

	klog.Errorf("Dropping SyncNode id %s out of the queue: %v", key, err)
	c.nodeQueue.Forget(key)
	utilruntime.HandleError(err)
}

func (c *Controller) syncNode(key string) error {
	klog.V(4).Infof("start patch %s", key)

	nodeName, metricName, err := splitMetaKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		klog.Errorf("Unable to list nodes (%v);", err)
		return err
	}

	newNode := node.DeepCopy()
	if isNotGPUNode(newNode) {
		return nil
	}
	gpuCount := GetGPUDeviceCountOfNode(newNode)
	for i := 0; i < gpuCount; i++ {
		err = c.annotatorNode(newNode, metricName, strconv.Itoa(i))
	}
	return err
}

func (c *Controller) annotatorNode(node *v1.Node, key, cardNum string) error {
	value, err := c.Prom.QueryLasterData(node.Name, key, cardNum)
	if len(value) == 0 || err != nil {
		klog.Errorf("QueryLasterData %s for node %s return value: %s , error: %v", key, node.Name, value, err)
		return errors.Errorf("QueryLasterData %s for node %s return value: %s , error: %v", key, node.Name, value, err)
	}
	card, err := strconv.Atoi(cardNum)
	if err != nil {
		return err
	}
	if key == dealer.GPUCoreUsagePriority {
		_, ok := c.dealer.GetCoreUsageLock(node.Name)
		if !ok {
			c.dealer.AddCoreUsage(node.Name)
		}
		c.dealer.UpdateCoreUsage(node.Name, value, c.getLocalTime(), card)
	} else {
		_, ok := c.dealer.GetMemoryUsageLock(node.Name)
		if !ok {
			c.dealer.AddMemoryUsage(node.Name)
		}
		c.dealer.UpdateMemoryUsage(node.Name, value, c.getLocalTime(), card)
	}
	return err
}

func (c *Controller) getLocalTime() string {
	loc, _ := time.LoadLocation(zone)
	now := time.Now().In(loc).Format(timeFormat)
	return now
}

func splitMetaKey(key string) (name, metricName string, err error) {
	parts := strings.Split(key, "/")
	switch len(parts) {
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("unexpected key format: %q", key)
	}
}

func isNotGPUNode(node *v1.Node) bool {
	if node.Labels["nvidia-device-enable"] == "enable" {
		return false
	}
	return true
}

func GetGPUDeviceCountOfNode(node *v1.Node) int {
	val, ok := node.Status.Capacity[ResourceGPUPercent]
	if !ok {
		return 0
	}
	return int(val.Value()) / GPUPercentEachCard
}