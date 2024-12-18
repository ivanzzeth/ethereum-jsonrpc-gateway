package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ivanzzeth/ethereum-jsonrpc-gateway/utils"
	"github.com/sirupsen/logrus"
)

var TimeoutError = fmt.Errorf("timeout error")
var AllUpstreamsFailedError = fmt.Errorf("all upstream requests are failed")

type Request struct {
	logger               *logrus.Entry
	chainId              uint64
	data                 *RequestData
	reqBytes             []byte
	isArchiveDataRequest bool
}

func getBlockNumberRequest(chainId uint64) *Request {
	res, err := newRequest(chainId, []byte(fmt.Sprintf(`{"params": [], "method": "eth_blockNumber", "id": %d, "jsonrpc": "2.0"}`, time.Now().Unix())))
	if err != nil {
		panic(err)
	}
	return res
}

func (r *Request) isOldTrieRequest(currentBlockNumber int) (res bool) {
	defer func() {
		r.isArchiveDataRequest = res
	}()

	method := r.data.Method
	var reqBlockNumber interface{}

	if method == "eth_blockNumber" {
		res = false
		return
	}

	if method == "eth_subscribe" || method == "eth_unsubscribe" || method == "trace_block" ||
		method == "trace_call" || method == "trace_callMany" || method == "trace_filter" ||
		method == "trace_transaction" || method == "eth_chainId" {
		res = true
		return
	}

	if method == "eth_getProof" || method == "eth_getStorageAt" {
		if len(r.data.Params) < 3 {
			return
		}

		reqBlockNumber = r.data.Params[2]
	} else if method == "eth_call" || method == "eth_createAccessList" ||
		method == "eth_estimateGas" || method == "eth_feeHistory" || method == "eth_getBalance" ||
		method == "eth_getCode" ||
		method == "eth_getTransactionCount" {
		if len(r.data.Params) < 2 {
			return
		}

		reqBlockNumber = r.data.Params[1]
	} else if method == "eth_getBlockByNumber" || method == "eth_getBlockReceipts" ||
		method == "eth_getBlockTransactionCountByNumber" || method == "eth_getUncleByBlockNumberAndIndex" ||
		method == "eth_getUncleCountByBlockNumber" || method == "eth_getTransactionByBlockNumberAndIndex" {
		if len(r.data.Params) >= 1 {
			return
		}

		reqBlockNumber = r.data.Params[0]
	} else if method == "eth_getLogs" {
		res = true
		return
	} else {
		res = true
		return
	}

	switch v := reqBlockNumber.(type) {
	case string:
		if v == "latest" || v == "pending" {
			return
		}
		n, _ := strconv.ParseInt(v, 0, 64)
		res = currentBlockNumber-int(n) > 100
	case int:
		logrus.Errorf("unknown %d", currentBlockNumber)
		res = currentBlockNumber-v > 100
	case float64:
		logrus.Errorf("unknown %d", currentBlockNumber)
		res = currentBlockNumber-int(v) > 100
	default:
		logrus.Errorf("unknown blocknumber %+v", v)
	}
	return
}

func newRequest(chainId uint64, reqBodyBytes []byte) (*Request, error) {
	logger := logrus.WithFields(logrus.Fields{"request_id": utils.RandStringRunes(8)})

	var data RequestData
	_ = json.Unmarshal(reqBodyBytes, &data)

	logger.Debugf("New, method: %s\n", data.Method)
	logger.Debugf("Request Body: %s\n", string(reqBodyBytes))

	req := &Request{
		logger:   logger,
		chainId:  chainId,
		data:     &data,
		reqBytes: reqBodyBytes,
	}

	// method limit, for directly external access
	err := req.valid()

	if err != nil {
		return req, err
	}

	return req, nil
}

func (r *Request) valid() error {

	if !currentRunningConfig.Configs[r.chainId].MethodLimitationEnabled {
		return nil
	}

	err := isValidCall(r.chainId, r.data)

	if err != nil {
		r.logger.Printf("not valid, skip\n")
		return err
	}

	return nil
}
