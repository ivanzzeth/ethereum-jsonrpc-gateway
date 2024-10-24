package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestBuildRunningConfigFromConfigNAIVE(t *testing.T) {
	var testConfigStr1 = `{
   "1337":{
      "_upstreams":"support http, https, ws, wss",
      "upstreams":[
         "https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
      ],
      "_oldTrieUrl":"for archive data, support http, https, or set empty string",
      "oldTrieUrl":"",
      "_strategy":"support NAIVE, RACE, FALLBACK",
      "strategy":"NAIVE",
      "_methodLimitationEnabled":"limit or not",
      "methodLimitationEnabled":true,
      "_allowedMethods":"can be ignored when set methodLimitationEnabled false",
      "allowedMethods":[
         "eth_blockNumber"
      ],
      "_contractWhitelist":"can be ignored when set methodLimitationEnabled false",
      "contractWhitelist":[]
   }
}`

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	currentRunningConfig, err := BuildRunningConfigFromConfig(context.Background(), config)
	if err != nil {
		logrus.Fatal(err)
	}

	currentRunningChainConfig := currentRunningConfig.Configs[1337]

	assert.Equal(t, true, currentRunningChainConfig.MethodLimitationEnabled)

	var testConfigStr2 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707",
			"https://test1.com"
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

	err = json.Unmarshal([]byte(testConfigStr2), config)
	if err != nil {
		t.Fatal(err)
	}

	assert.Panics(t, func() { BuildRunningConfigFromConfig(context.Background(), config) })
}

func TestBuildRunningConfigFromConfigRACE(t *testing.T) {
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

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	currentRunningConfig, err := BuildRunningConfigFromConfig(context.Background(), config)

	if err != nil {
		logrus.Fatal(err)
	}

	chainId := uint64(1337)

	assert.Equal(t, true, currentRunningConfig.Configs[chainId].MethodLimitationEnabled)

	var testConfigStr2 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
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

	err = json.Unmarshal([]byte(testConfigStr2), config)
	if err != nil {
		t.Fatal(err)
	}

	assert.Panics(t, func() { BuildRunningConfigFromConfig(context.Background(), config) })
}

func TestBuildRunningConfigFromConfigFALLBACK(t *testing.T) {
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
			"contractWhitelist": ["0xc2c57336e01695D34F8012f6c0d250baB2Dd38Da"]
		}
	}`

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	currentRunningConfig, err := BuildRunningConfigFromConfig(context.Background(), config)

	if err != nil {
		logrus.Fatal(err)
	}

	chainId := uint64(1337)

	assert.Equal(t, true, currentRunningConfig.Configs[chainId].MethodLimitationEnabled)

	var testConfigStr2 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
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

	err = json.Unmarshal([]byte(testConfigStr2), config)
	if err != nil {
		t.Fatal(err)
	}

	assert.Panics(t, func() { BuildRunningConfigFromConfig(context.Background(), config) })
}

func TestBuildRunningConfigFromConfigOldTreeUrl(t *testing.T) {
	var testConfigStr1 = `{
		"1337":{
			"_upstreams":"support http, https, ws, wss",
			"upstreams":[
				"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
			],
			"_oldTrieUrl":"for archive data, support http, https, or set empty string",
			"oldTrieUrl":"",
			"_strategy":"support NAIVE, RACE, FALLBACK",
			"strategy":"NAIVE",
			"_methodLimitationEnabled":"limit or not",
			"methodLimitationEnabled":true,
			"_allowedMethods":"can be ignored when set methodLimitationEnabled false",
			"allowedMethods":[
				"eth_blockNumber"
			],
			"_contractWhitelist":"can be ignored when set methodLimitationEnabled false",
			"contractWhitelist":[
				"0xc2c57336e01695D34F8012f6c0d250baB2Dd38Da"
			]
		}
	}`

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("config: %v", *config)

	chainId := uint64(1337)

	currentRunningConfig, err := BuildRunningConfigFromConfig(context.Background(), config)
	if err != nil {
		logrus.Fatal(err)
	}

	assert.Equal(t, true, currentRunningConfig.Configs[chainId].MethodLimitationEnabled)

	var testConfigStr2 = `{
		"1337": {
			"_upstreams": "support http, https, ws, wss",
			"upstreams": [
			"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
			],

			"_oldTrieUrl": "for archive data, support http, https, or set empty string",
			"oldTrieUrl": "https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707x",

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

	err = json.Unmarshal([]byte(testConfigStr2), config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildRunningConfigFromConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707x", (*config)[chainId].OldTrieUrl)
}

func initTestConfig(t *testing.T) {
	var testConfigStr1 = `{
		"1337":{
			"_upstreams":"support http, https, ws, wss",
			"upstreams":[
				"https://ropsten.infura.io/v3/83438c4dcf834ceb8944162688749707"
			],
			"_oldTrieUrl":"for archive data, support http, https, or set empty string",
			"oldTrieUrl":"",
			"_strategy":"support NAIVE, RACE, FALLBACK",
			"strategy":"NAIVE",
			"_methodLimitationEnabled":"limit or not",
			"methodLimitationEnabled":true,
			"_allowedMethods":"can be ignored when set methodLimitationEnabled false",
			"allowedMethods":[
				"eth_blockNumber"
			],
			"_contractWhitelist":"can be ignored when set methodLimitationEnabled false",
			"contractWhitelist":[
				"0xc2c57336e01695D34F8012f6c0d250baB2Dd38Da"
			]
		}
	}`

	config := NewConfig()

	err := json.Unmarshal([]byte(testConfigStr1), config)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("config: %v", *config)

	_, err = BuildRunningConfigFromConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
}
