package prometheus

import (
	promApiV1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

//PromAPIS prometheus apis
type PromAPIS interface {
	QueryLasterData(nodeName, key, cardNum string) (metricValue string, err error)
}

//PromConfig prometheus config
type PromConfig struct {
	ApiClient    promApiV1.API
	InstancePort string
}
