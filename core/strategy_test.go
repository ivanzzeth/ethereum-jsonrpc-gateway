package core

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNaiveProxyHandle(t *testing.T) {
	var testConfigStr1 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
			],
		
			"_strategy": "support NAIVE, RACE, FALLBACK",
			"strategy": "NAIVE",
		
			"_methodLimitationEnabled": "limit or not",
			"methodLimitationEnabled": true,
		
			"_allowedMethods": "can be ignored when set methodLimitationEnabled false",
			"allowedMethods": ["eth_blockNumber"],
		
			"_contractWhitelist": "can be ignored when set methodLimitationEnabled false",
			"contractWhitelist": []
		}
	}`

	ctx := context.Background()

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	currentRunningConfig, err = BuildRunningConfigFromConfig(ctx, config)

	if err != nil {
		logrus.Fatal(err)
	}

	chainId := uint64(1337)

	reqBodyBytes1 := []byte(fmt.Sprintf(`{"params": [], "method": "eth_blockNumber", "id": %d, "jsonrpc": "2.0"}`, time.Now().Unix()))
	req1, err := newRequest(chainId, reqBodyBytes1)

	if err != nil {
		logrus.Fatal(err)
	}

	proxy := newNaiveProxy()

	bts, err := proxy.handle(req1)

	assert.Equal(t, nil, err)

	assert.IsType(t, []byte{}, bts)
}

func TestRaceProxyHandle(t *testing.T) {
	var testConfigStr1 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707",
			"https://test1.com"
			],
			
			"_strategy": "support NAIVE, RACE, FALLBACK",
			"strategy": "RACE",
		
			"_methodLimitationEnabled": "limit or not",
			"methodLimitationEnabled": true,
		
			"_allowedMethods": "can be ignored when set methodLimitationEnabled false",
			"allowedMethods": ["eth_blockNumber"],
		
			"_contractWhitelist": "can be ignored when set methodLimitationEnabled false",
			"contractWhitelist": []
		}
	}`

	ctx := context.Background()

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	currentRunningConfig, err = BuildRunningConfigFromConfig(ctx, config)

	if err != nil {
		logrus.Fatal(err)
	}

	chainId := uint64(1337)

	reqBodyBytes1 := []byte(fmt.Sprintf(`{"params": [], "method": "eth_blockNumber", "id": %d, "jsonrpc": "2.0"}`, time.Now().Unix()))
	req1, err := newRequest(chainId, reqBodyBytes1)

	if err != nil {
		logrus.Fatal(err)
	}

	proxy := newNaiveProxy()

	bts, err := proxy.handle(req1)

	assert.Equal(t, nil, err)

	assert.IsType(t, []byte{}, bts)
}

func TestNewFallbackProxy(t *testing.T) {
	initTestConfig(t)
	assert.IsType(t, &FallbackProxy{}, newFallbackProxy())
}

func TestFallbackProxyHandle(t *testing.T) {
	var testConfigStr1 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707",
			"https://test1.com"
			],
		
			"_strategy": "support NAIVE, RACE, FALLBACK",
			"strategy": "FALLBACK",
		
			"_methodLimitationEnabled": "limit or not",
			"methodLimitationEnabled": true,
		
			"_allowedMethods": "can be ignored when set methodLimitationEnabled false",
			"allowedMethods": ["eth_blockNumber"],
		
			"_contractWhitelist": "can be ignored when set methodLimitationEnabled false",
			"contractWhitelist": []
		}
	}`

	ctx := context.Background()

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	currentRunningConfig, err = BuildRunningConfigFromConfig(ctx, config)

	if err != nil {
		logrus.Fatal(err)
	}

	chainId := uint64(1337)

	reqBodyBytes1 := []byte(fmt.Sprintf(`{"params": [], "method": "eth_blockNumber", "id": %d, "jsonrpc": "2.0"}`, time.Now().Unix()))
	req1, err := newRequest(chainId, reqBodyBytes1)

	if err != nil {
		logrus.Fatal(err)
	}

	proxy := newFallbackProxy()

	bts, err := proxy.handle(req1)

	assert.Equal(t, nil, err)

	assert.IsType(t, []byte{}, bts)
}
