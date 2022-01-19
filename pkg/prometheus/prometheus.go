package prometheus

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	promApi "github.com/prometheus/client_golang/api"
	promApiV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/klog"
)

//NewPromConfig new prom config
func NewPromConfig(url string, port string) PromAPIS {

	promConfig := promApi.Config{Address: url}
	client, err := promApi.NewClient(promConfig)
	if err != nil {
		klog.Errorf("Error creating promapi client: %v\n", err)
		panic("Error creating promapi client: " + err.Error())
	}

	apiClient := promApiV1.NewAPI(client)

	return &PromConfig{
		ApiClient:    apiClient,
		InstancePort: port,
	}
}

func (p *PromConfig) queryDataHelper(querySelect string) (metricValue string, err error) {
	klog.V(4).Infof("starting query Prometheus, req: %s", querySelect)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := p.ApiClient.Query(ctx, querySelect, time.Now())
	if err != nil {
		klog.Errorf("Error querying Prometheus: %v", err)
		//os.Exit(1)
		return "", err
	}

	if len(warnings) > 0 {
		klog.Warningf("Warnings querying Prometheus: %+v\n", warnings)
		return "", err
	}

	if result.Type() != model.ValVector {
		klog.Warningf("result.Type() is %+v", result.Type())
		return "", err
	}

	for _, elem := range result.(model.Vector) {
		if float64(elem.Value) < float64(0) || math.IsNaN(float64(elem.Value)) {
			elem.Value = 0
		}
		metricValue = strconv.FormatFloat(float64(elem.Value), 'f', 5, 64)
	}

	return metricValue, nil
}

//QueryLasterData query data
func (p *PromConfig) QueryLasterData(nodeName, key, cardNum string) (metricValue string, err error) {
	var val string
	label := fmt.Sprintf("node=~\"%s\",card=\"%s\"", nodeName, cardNum)
	querySelect := key + "{" + label + "}" + " /100"
	val, err = p.queryDataHelper(querySelect)
	if len(val) == 0 || err != nil {
		klog.V(4).Infof("retry with instance only IP")
		label := fmt.Sprintf("node=\"%s\",cardNode=\"%s\"", nodeName, cardNum)
		querySelect := key + "{" + label + "}" + " /100"
		val, err = p.queryDataHelper(querySelect)
		if err != nil {
			return "", nil
		}
	}
	return val, nil
}

