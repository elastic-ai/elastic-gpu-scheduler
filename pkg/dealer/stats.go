package dealer

import (
	"errors"
	"io/ioutil"
	"time"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/klog"
)

//GetPolicyFromFile get config
func GetPolicyFromFile(path string) *Policy {
	policy := new(Policy)

	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		klog.Errorf("getPredicatePolicy ioutil.ReadFile policy yaml error: %v", err)
		panic("getPredicatePolicy ioutil.ReadFile policy yaml error")
	}

	err = yaml.Unmarshal(yamlFile, policy)
	if err != nil {
		klog.Errorf("Unmarshal policy yaml error: %v", err)
	}

	return policy
}

func inUpdateTimePeriod(updatetimeStr string, activeDuration time.Duration) bool {
	klog.V(6).Infof("updatetimeStr %s", updatetimeStr)
	if len(updatetimeStr) < 5 {
		klog.Errorf("updatetimeStr len < 5")
		return false
	}
	loc, err := time.LoadLocation("Asia/Shanghai") //设置时区
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	nodeStateUpdateTime, err := time.ParseInLocation(timeFormat, updatetimeStr, loc)
	if err != nil {
		klog.Errorf("nodeStateUpdateTime ParseInLocation error")
		return false
	}

	now := time.Now().In(loc)
	updatetime := nodeStateUpdateTime.Add(activeDuration)
	klog.V(6).Infof("now %v , updatetime %v", now, updatetime)
	if now.Before(updatetime) {
		return true
	}
	klog.Info("Updatetime is %v and now is %v",updatetime, now)
	return false
}

func getActiveDuration(syncPeriodList []Period, name string) (time.Duration, error) {
	for _, period := range syncPeriodList {
		if period.Name == name {
			if period.Period != 0 {
				return period.Period + ExtenderAtivePeriod, nil
			}
		}
	}
	return 0, errors.New("error get activeDuration")
}
