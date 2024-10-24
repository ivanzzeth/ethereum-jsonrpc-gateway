package core

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ivanzzeth/ethereum-jsonrpc-gateway/utils"
	"github.com/sirupsen/logrus"
)

type IStrategy interface {
	handle(*Request) ([]byte, error)
}

var _ IStrategy = &NaiveProxy{}
var _ IStrategy = &RaceProxy{}
var _ IStrategy = &FallbackProxy{}

// var _ IStrategy = &LoadBalanceFallbackProxy{}

type NaiveProxy struct{}

func newNaiveProxy() *NaiveProxy {
	return &NaiveProxy{}
}

func (p *NaiveProxy) handle(req *Request) ([]byte, error) {
	upstream := currentRunningConfig.Configs[req.chainId].Upstreams[0]
	bts, err := upstream.handle(req)

	if err != nil {
		return nil, err
	}

	return bts, err
}

type RaceProxy struct{}

func newRaceProxy() *RaceProxy {
	return &RaceProxy{}
}

func (p *RaceProxy) handle(req *Request) ([]byte, error) {
	startAt := time.Now()

	defer func() {
		logrus.Debugf("geth_gateway %f", float64(time.Since(startAt))/1000000)
	}()

	successfulResponse := make(chan []byte, len(currentRunningConfig.Configs[req.chainId].Upstreams))
	failedResponse := make(chan []byte, len(currentRunningConfig.Configs[req.chainId].Upstreams))
	errorResponseUpstreams := make(chan Upstream, len(currentRunningConfig.Configs[req.chainId].Upstreams))

	for _, upstream := range currentRunningConfig.Configs[req.chainId].Upstreams {
		go func(upstream Upstream) {
			defer func() {
				if err := recover(); err != nil {
					req.logger.Debugf("%v Upstream %s failed, err: %v\n", time.Now().Sub(startAt), upstream, err)
					errorResponseUpstreams <- upstream
				}
			}()

			bts, err := upstream.handle(req)

			if err != nil {
				req.logger.Debugf("%vms Upstream: %v, Error: %v\n", time.Now().Sub(startAt), upstream, err)
				failedResponse <- nil
				return
			}

			resBody := strings.TrimSpace(string(bts))

			diff := time.Now().Sub(startAt)
			if utils.NoErrorFieldInJSON(resBody) {
				req.logger.Debugf("%v Upstream: %v Success, Body: %v\n", diff, upstream, resBody)
				successfulResponse <- bts
			} else {
				req.logger.Debugf("%v Upstream: %v Failed, Body: %v\n", diff, upstream, resBody)
				failedResponse <- bts
			}
		}(upstream)
	}

	errorCount := 0

	for errorCount < len(currentRunningConfig.Configs[req.chainId].Upstreams) {
		select {
		case <-time.After(time.Second * 10):
			req.logger.Debugf("%v Final Timeout\n", time.Now().Sub(startAt))
			return nil, TimeoutError
		case res := <-successfulResponse:
			req.logger.Debugf("%v Final Success\n", time.Now().Sub(startAt))
			return res, nil
		case res := <-failedResponse:
			return res, nil
		case <-errorResponseUpstreams:
			errorCount++
		}
	}

	req.logger.Errorf("%v Final Failed\n", time.Now().Sub(startAt))

	logrus.Errorf("geth_gateway_fail")

	return nil, AllUpstreamsFailedError
}

type FallbackProxy struct {
	status sync.Map
}

type FallbackStatus struct {
	currentUpstreamIndex *atomic.Value
	upsteamStatus        *sync.Map
}

func newFallbackProxy() *FallbackProxy {
	// logrus.Infof("using fallback proxy for chain")

	p := &FallbackProxy{}

	for chainId, cfg := range currentRunningConfig.Configs {
		v := &atomic.Value{}
		v.Store(0)
		status := &FallbackStatus{
			currentUpstreamIndex: v,
			upsteamStatus:        &sync.Map{},
		}
		for i := 0; i < len(cfg.Upstreams); i++ {
			status.upsteamStatus.Store(i, true)
		}

		logrus.Infof("fallback proxy setup chainId %v", chainId)
		p.status.Store(chainId, status)
	}

	// logrus.Infof("setup fallback proxy ended, chains %v, config: %v", len(currentRunningConfig.Configs), *currentRunningConfig)

	return p
}

func (p *FallbackProxy) handle(req *Request) ([]byte, error) {
	statusVal, ok := p.status.Load(req.chainId)
	if !ok {
		return nil, fmt.Errorf("chain not supported")
	}

	status := statusVal.(*FallbackStatus)

	cfg := currentRunningConfig.Configs[req.chainId]

	for i := 0; i < len(cfg.Upstreams); i++ {
		index := status.currentUpstreamIndex.Load().(int)

		value, _ := status.upsteamStatus.Load(index)
		isUpstreamValid := value.(bool)

		if isUpstreamValid {
			bts, err := cfg.Upstreams[index].handle(req)

			retry := func() {
				nextUpstreamIndex := int(math.Mod(float64(index+1), float64(len(cfg.Upstreams))))
				status.currentUpstreamIndex.Store(nextUpstreamIndex)
				status.upsteamStatus.Store(i, false)

				logrus.Infof("upstream %d return err, switch to %d", index, nextUpstreamIndex)

				go func(i int) {
					<-time.After(5 * time.Second)
					status.upsteamStatus.Store(i, true)
				}(index)
			}
			if err != nil {
				retry()
				continue
			} else {
				resp := &JsonRpcResponse{}
				err = json.Unmarshal(bts, resp)
				if err != nil {
					logrus.Errorf("JsonRpcResponse unmarsharling failed: %v", err)
					retry()
					continue
				}
				if resp.Err.Code != 0 {
					retry()
					continue
				}
				return bts, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid upstream")
}

type LoadBalanceFallbackProxy struct {
	status sync.Map
}

func newLoadBalanceFallbackProxy() *LoadBalanceFallbackProxy {
	// logrus.Infof("using load balancing proxy for chain")

	p := &LoadBalanceFallbackProxy{}

	for chainId, cfg := range currentRunningConfig.Configs {
		v := &atomic.Value{}
		v.Store(0)
		status := &FallbackStatus{
			currentUpstreamIndex: v,
			upsteamStatus:        &sync.Map{},
		}
		for i := 0; i < len(cfg.Upstreams); i++ {
			status.upsteamStatus.Store(i, true)
		}

		// logrus.Infof("load balancing proxy setup chainId %v", chainId)
		p.status.Store(chainId, status)
	}

	// logrus.Infof("setup load balancing proxy ended, chains %v, config: %v", len(currentRunningConfig.Configs), *currentRunningConfig)

	return p
}

func (p *LoadBalanceFallbackProxy) handle(req *Request) ([]byte, error) {
	statusVal, ok := p.status.Load(req.chainId)
	if !ok {
		return nil, fmt.Errorf("chain not supported")
	}

	status := statusVal.(*FallbackStatus)

	cfg := currentRunningConfig.Configs[req.chainId]

	initialIndex := status.currentUpstreamIndex.Load().(int)

	for i := 0; i < len(cfg.Upstreams); i++ {
		index := status.currentUpstreamIndex.Load().(int)
		if i != 0 && index == initialIndex {
			break
		}
		value, _ := status.upsteamStatus.Load(index)
		isUpstreamValid := value.(bool)

		if isUpstreamValid {
			bts, err := cfg.Upstreams[index].handle(req)

			switchFunc := func() {
				nextUpstreamIndex := int(math.Mod(float64(index+1), float64(len(cfg.Upstreams))))
				status.currentUpstreamIndex.Store(nextUpstreamIndex)
				logrus.Infof("upstream %d load balancing, then switch to %d", index, nextUpstreamIndex)
			}

			switchFunc()

			if err != nil {
				continue
			} else {
				resp := &JsonRpcResponse{}
				err = json.Unmarshal(bts, resp)
				if err != nil {
					logrus.Errorf("JsonRpcResponse unmarsharling failed: %v", err)
					switchFunc()
					continue
				}
				if resp.Err.Code != 0 {
					switchFunc()
					continue
				}
				return bts, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid upstream")
}
