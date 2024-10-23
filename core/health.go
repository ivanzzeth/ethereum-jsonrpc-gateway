package core

import (
	"fmt"
	"sync"
	"time"
)

type NodeInfo struct {
	RpcUrl  string `json:"rpcUrl"` // only first 20 chars
	Latency string `json:"latency"`
	IsAlive bool   `json:"isAlive"`
}

type HealthInfo map[uint64][]NodeInfo

var cachedHealthInfo HealthInfo = make(map[uint64][]NodeInfo)
var nextUpdateTime time.Time
var healthInfoUpdateLocker sync.Mutex

func getHealthInfo() HealthInfo {
	if nextUpdateTime.Before(time.Now()) {
		healthInfoUpdateLocker.Lock()

		for chainId, cfg := range currentRunningConfig.Configs {
			nodesInfo := []NodeInfo{}
			for _, up := range cfg.Upstreams {
				url := up.getRpcUrl()
				if len(url) > 30 {
					url = url[:30]
				}

				nodesInfo = append(nodesInfo, NodeInfo{
					RpcUrl:  url,
					Latency: fmt.Sprintf("%s", time.Duration(up.getLatancy())),
					IsAlive: up.isAlive(),
				})
			}

			cachedHealthInfo[chainId] = nodesInfo
		}

		healthInfoUpdateLocker.Unlock()

		nextUpdateTime = time.Now().Add(1 * time.Minute)
	}

	return cachedHealthInfo
}
