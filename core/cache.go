package core

import (
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

const CacheSize = 20000

var cache *lru.TwoQueueCache
var cacheOnce sync.Once

func getCache() *lru.TwoQueueCache {
	cacheOnce.Do(func() {
		cache = initCache(CacheSize)
	})

	return cache
}

func initCache(size int) *lru.TwoQueueCache {
	cache, err := lru.New2Q(size)
	if err != nil {
		panic(fmt.Errorf("init cache failed: %v", err))
	}

	return cache
}
