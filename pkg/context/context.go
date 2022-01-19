package context

import (
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/dealer"
	"k8s.io/klog"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"
)

type DSContext struct {
	PolicyConfigPath string
	LastModifyTime   int64
	Mutex            sync.RWMutex
	policy           *dealer.Policy
}

func NewDSContext(PolicyConfigPath string) *DSContext {
	return &DSContext{
		PolicyConfigPath:PolicyConfigPath,
	}
}

//Start start
func (ctx *DSContext) Start() {
	klog.Infof("context start ...")
	ctx.Mutex.Lock()
	ctx.policy = dealer.GetPolicyFromFile(ctx.PolicyConfigPath)
	f, _ := os.Stat(ctx.PolicyConfigPath)
	curModifyTime := f.ModTime().Unix()
	ctx.LastModifyTime = curModifyTime
	ctx.Mutex.Unlock()
	go ctx.AutoReload()

}

func (ctx *DSContext) GetPolicySpec() dealer.PolicySpec {
	ctx.Mutex.RLock()
	defer ctx.Mutex.RUnlock()
	return ctx.policy.Spec
}

func (c *DSContext) AutoReload() {
	ticker := time.NewTicker(time.Second * 3)
	for {
		select {
		case <-ticker.C:
			f, _ := os.Stat(c.PolicyConfigPath)
			curModifyTime := f.ModTime().Unix()
			if curModifyTime > c.LastModifyTime {
				c.Mutex.Lock()
				c.policy = dealer.GetPolicyFromFile(c.PolicyConfigPath)
				c.LastModifyTime = curModifyTime
				c.Mutex.Unlock()
			}
		}
	}
}
