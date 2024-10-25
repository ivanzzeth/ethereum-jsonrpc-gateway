package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Config map[uint64]ChainConfig

func NewConfig() *Config {
	c := Config(make(map[uint64]ChainConfig))
	return &c
}

type ChainConfig struct {
	Upstreams               []string `json:"upstreams"`
	OldTrieUrl              string   `json:"oldTrieUrl"`
	Strategy                string   `json:"strategy"`
	MethodLimitationEnabled bool     `json:"methodLimitationEnabled"`
	AllowedMethods          []string `json:"allowedMethods"`
	ContractWhitelist       []string `json:"contractWhitelist"`
}

type RunningConfig struct {
	ctx     context.Context
	stop    context.CancelFunc
	Configs map[uint64]*RunningChainConfig
}

func (c *RunningConfig) close() {
	c.stop()
}

func (c *RunningConfig) healthCheck() {
	go func() {
		// TODO: Configurable
		ticker := time.NewTicker(2 * time.Minute)

		check := func() {
			logrus.Infof("healthCheck started...")
			for _, cfg := range c.Configs {
				cfg.healthCheck()

				time.Sleep(10 * time.Second)
			}
		}

		check()

		for {
			select {
			case <-ticker.C:
				check()
			case <-c.ctx.Done():
				logrus.Infof("healthCheck closed...")
				return
			}
		}
	}()
}

type RunningChainConfig struct {
	Upstreams               []Upstream
	Strategy                IStrategy
	MethodLimitationEnabled bool
	allowedMethods          map[string]bool
	allowedCallContracts    map[string]bool

	updateLocker sync.RWMutex
}

func (c *RunningChainConfig) healthCheck() {
	c.updateLocker.Lock()
	defer c.updateLocker.Unlock()

	var wg sync.WaitGroup
	for _, up := range c.Upstreams {
		wg.Add(1)

		go func(up Upstream) {
			up.updateBlockNumber()
			wg.Done()
		}(up)
	}

	wg.Wait()

	sort.Slice(c.Upstreams, func(i, j int) bool {
		return c.Upstreams[i].getLatancy() <= c.Upstreams[j].getLatancy()
	})

	logrus.Infof("running chain upstreams updated")
}

func NewRunningConfig(ctx context.Context, cfg *Config) (*RunningConfig, error) {
	oldOne := currentRunningConfig
	if oldOne != nil {
		defer oldOne.close()
	}

	ctx, stop := context.WithCancel(ctx)

	rcfg := &RunningConfig{
		ctx:     ctx,
		stop:    stop,
		Configs: make(map[uint64]*RunningChainConfig),
	}
	currentRunningConfig = rcfg

	for chainId, chainCfg := range *cfg {
		rcfg.Configs[chainId] = &RunningChainConfig{}

		rcfg.Configs[chainId].updateLocker.Lock()
		defer rcfg.Configs[chainId].updateLocker.Unlock()

		for _, url := range chainCfg.Upstreams {

			var primaryUrl string
			var oldTrieUrl string

			if chainCfg.OldTrieUrl != "" {
				primaryUrl = url
				oldTrieUrl = chainCfg.OldTrieUrl
			} else {
				primaryUrl = url
				oldTrieUrl = url
			}

			rcfg.Configs[chainId].Upstreams = append(rcfg.Configs[chainId].Upstreams, newUpstream(ctx, chainId, primaryUrl, oldTrieUrl))
		}

		if len(rcfg.Configs[chainId].Upstreams) == 0 {
			return nil, fmt.Errorf("need upstreams")
		}

		go rcfg.healthCheck()

		switch chainCfg.Strategy {
		case "NAIVE":
			if len(rcfg.Configs[chainId].Upstreams) > 1 {
				panic(fmt.Errorf("naive proxy strategy require exact 1 upstream"))
			}
			rcfg.Configs[chainId].Strategy = newNaiveProxy()
		case "RACE":
			if len(rcfg.Configs[chainId].Upstreams) < 2 {
				panic(fmt.Errorf("race proxy strategy require more than 1 upstream"))
			}
			rcfg.Configs[chainId].Strategy = newRaceProxy()
		case "FALLBACK":
			if len(rcfg.Configs[chainId].Upstreams) < 2 {
				panic(fmt.Errorf("fallback proxy strategy require more than 1 upstream"))
			}
			rcfg.Configs[chainId].Strategy = newFallbackProxy()
		case "BALANCING":
			if len(rcfg.Configs[chainId].Upstreams) < 2 {
				panic(fmt.Errorf("loadbalance proxy strategy require more than 1 upstream"))
			}
			rcfg.Configs[chainId].Strategy = newLoadBalanceFallbackProxy()
		default:
			return nil, fmt.Errorf("blank of unsupported strategy: %s", chainCfg.Strategy)
		}

		(rcfg.Configs[chainId]).MethodLimitationEnabled = chainCfg.MethodLimitationEnabled

		rcfg.Configs[chainId].allowedMethods = make(map[string]bool)
		for i := 0; i < len(chainCfg.AllowedMethods); i++ {
			rcfg.Configs[chainId].allowedMethods[chainCfg.AllowedMethods[i]] = true
		}

		rcfg.Configs[chainId].allowedCallContracts = make(map[string]bool)
		for i := 0; i < len(chainCfg.ContractWhitelist); i++ {
			rcfg.Configs[chainId].allowedCallContracts[strings.ToLower(chainCfg.ContractWhitelist[i])] = true
		}
	}

	return rcfg, nil
}

var currentConfigString string = ""
var currentRunningConfig *RunningConfig

func LoadConfig(ctx context.Context, quit chan bool) {
	reloadConfig := func() {
		config := NewConfig()

		logrus.Debugf("load config from file")
		bts, err := ioutil.ReadFile("./config.json")

		if err != nil {
			if currentConfigString == "" {
				logrus.Fatal(err)
			} else {
				logrus.Warn("hot read config err, use old config")
			}
		}

		if currentConfigString == "" || string(bts) != currentConfigString {
			_ = json.Unmarshal(bts, config)

			logrus.Infof("reloading config: %v", *config)
			currentRunningConfig, err = BuildRunningConfigFromConfig(ctx, config)

			if err != nil {
				if currentConfigString == "" {
					logrus.Fatal(err)
				} else {
					logrus.Warn("hot build config err, use old config")

				}
			}

			logrus.Infof("reloading running config: %v", currentRunningConfig.Configs)

			currentConfigString = string(bts)
		}
	}

	// loading on init.
	reloadConfig()

	ticker := time.NewTicker(3 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				reloadConfig()
			case <-quit:
				logrus.Info("quit loop config")
				ticker.Stop()
				return
			}
		}
	}()
}

func BuildRunningConfigFromConfig(parentContext context.Context, cfg *Config) (*RunningConfig, error) {
	return NewRunningConfig(parentContext, cfg)
}
