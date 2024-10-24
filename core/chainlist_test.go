package core

import "testing"

func TestChainlist(t *testing.T) {
	chainlist := getChainList()
	t.Logf("chainlist: %v", chainlist)
}
