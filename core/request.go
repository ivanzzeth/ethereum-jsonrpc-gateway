package core

import (
	"encoding/json"
	"fmt"
	"github.com/HydroProtocol/ethereum-jsonrpc-gateway/utils"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"strconv"
	"time"
)

var TimeoutError = fmt.Errorf("timeout error")
var AllUpstreamsFailedError = fmt.Errorf("all upstream requests are failed")

type Request struct {
	logger               *log.Logger
	data                 *RequestData
	reqBytes             []byte
	isArchiveDataRequest bool
}

func getBlockNumberRequest() *Request {
	res, _ := newRequest([]byte(fmt.Sprintf(`{"params": [], "method": "eth_blockNumber", "id": %d, "jsonrpc": "2.0"}`, time.Now().Unix())))
	return res
}

func (r *Request) isOldTrieRequest(currentBlockNumber int) (res bool) {
	defer func() {
		r.isArchiveDataRequest = res
	}()

	if currentBlockNumber == 0 {
		res = false
		return
	}

	method := r.data.Method

	if method != "eth_call" && method != "eth_getBalance" {
		res = false
		return
	}

	if len(r.data.Params) != 2 {
		res = false
		return
	}

	reqBlockNumber := r.data.Params[1]

	switch v := reqBlockNumber.(type) {
	case string:
		if v == "latest" || v == "pending" {
			res = false
			return
		}
		n, _ := strconv.ParseInt(v, 0, 64)
		res = currentBlockNumber-int(n) > 100
	case int:
		res = currentBlockNumber-int(v) > 100
	default:
		logrus.Errorf("unknown blocknumber %+v", v)
		res = false
	}
	return
}

func newRequest(reqBodyBytes []byte) (*Request, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("[id: %v] ", utils.RandStringRunes(8)), log.LstdFlags)

	var data RequestData
	_ = json.Unmarshal(reqBodyBytes, &data)

	//logger.Printf("New, method: %s\n", data.Method)
	//logger.Printf("Request Body: %s\n", string(reqBodyBytes))

	req := &Request{
		logger:   logger,
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

	if !currentRunningConfig.MethodLimitationEnabled {
		return nil
	}

	err := isValidCall(r.data)

	if err != nil {
		r.logger.Printf("not valid, skip\n")
		return err
	}

	return nil
}