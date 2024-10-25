package core

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var httpClient *http.Client

const (
	maxIdleConnections int = 200
	requestTimeout     int = 10
)

func init() {
	httpClient = createHTTPClient()
	rand.Seed(time.Now().UnixNano())
}

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: maxIdleConnections,
		},
		Timeout: time.Duration(requestTimeout) * time.Second,
	}
}

type Server struct{}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Server) ServerWS(chainId uint64, conn *websocket.Conn) error {
	defer conn.Close()

	for {
		messageType, r, err := conn.NextReader()
		if err != nil {
			return err
		}

		w, err := conn.NextWriter(messageType)

		if err != nil {
			return err
		}

		reqBodyBytes, _ := ioutil.ReadAll(r)
		proxyRequest, err := newRequest(chainId, reqBodyBytes)

		if err != nil {
			return err
		}

		Count(proxyRequest.data.Method)

		bts, err := currentRunningConfig.Configs[chainId].Strategy.handle(proxyRequest)

		if err != nil {
			bts = getErrorResponseBytes(proxyRequest.data.ID, err.Error())
		}

		if _, err := w.Write(bts); err != nil {
			return err
		}

		if err := w.Close(); err != nil {
			return err
		}
	}
}

func getErrorResponseBytes(id interface{}, reason interface{}) []byte {
	bts, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    -32602,
			"message": reason,
		},
	})

	return bts
}

// Support path:
// 1. /ws/{chainId}
// 2. /http/{chainId}
func (h *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error
	if strings.HasPrefix(req.URL.Path, "/ws") {
		chainIdStr := strings.Replace(req.URL.Path, "/ws/", "", 1)
		chainId, err := strconv.ParseUint(chainIdStr, 10, 64)
		if err != nil {
			w.WriteHeader(400)
			_, _ = w.Write([]byte("Invalid ChainId"))
			Count("bad_request")
			return
		}

		conn, err := upgrader.Upgrade(w, req, nil)

		if err != nil {
			logrus.Error(err)
			return
		}

		_ = h.ServerWS(chainId, conn)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if req.URL.Path == "/health" {
		info := getHealthInfo()
		infoBts, _ := json.Marshal(info)
		_, _ = w.Write(infoBts)
		return
	}

	if req.Method != http.MethodPost {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("Method Should Be POST"))
		Count("bad_request")
		return
	}

	if !strings.HasPrefix(req.URL.Path, "/http") {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("protocol Should Be http or ws"))
		Count("bad_request")
		return
	}

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	chainIdStr := strings.Replace(req.URL.Path, "/http/", "", 1)
	chainId, err := strconv.ParseUint(chainIdStr, 10, 64)
	if err != nil {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("Invalid ChainId"))
		Count("bad_request")
		return
	}

	startTime := time.Now()
	reqBodyBytes, _ := ioutil.ReadAll(req.Body)
	proxyRequest, err := newRequest(chainId, reqBodyBytes)
	Count(proxyRequest.data.Method)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(getErrorResponseBytes(proxyRequest.data.ID, err.Error()))
		logrus.Errorf("Req from %s %s 500 %s", req.RemoteAddr, proxyRequest.data.Method, err.Error())
		return
	}

	rpcReqKey := &ReqCacheKey{
		ChainId: chainId,
		RequestData: RequestData{
			JsonRpc: proxyRequest.data.JsonRpc,
			Method:  proxyRequest.data.Method,
			Params:  proxyRequest.data.Params,
		},
	}
	reqKey, err := json.Marshal(rpcReqKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(getErrorResponseBytes(proxyRequest.data.ID, err.Error()))
		logrus.Errorf("Req from %s %s 500 %s", req.RemoteAddr, proxyRequest.data.Method, err.Error())
		return
	}

	if val, ok := getCache().Get(string(reqKey)); ok {
		resp := val.([]byte)
		_, _ = w.Write(resp)

		Count("hit_cache")
		Count("hit_cache_" + proxyRequest.data.Method)

		trimedResp := resp
		if len(trimedResp) > 200 {
			trimedResp = trimedResp[:200]
		}
		logrus.Infof("Req for chain %d from %s %s(%v) 200 hits cache: %v", chainId, req.RemoteAddr,
			proxyRequest.data.Method, string(proxyRequest.reqBytes), string(trimedResp))

		return
	}

	Count("miss_cache")

	var btsResp []byte

	defer func() {
		costInMs := time.Since(startTime).Nanoseconds() / 1000000
		if costInMs > 5000 {
			logrus.Infof("slow request, method: %s, cost: %d", proxyRequest.data.Method, costInMs)
		}
		Time(proxyRequest.data.Method, float64(costInMs))

		if err == nil {
			// cache result
			jsonRpcResp := &JsonRpcResponse{}
			err = json.Unmarshal(btsResp, jsonRpcResp)
			if err == nil {
				if proxyRequest.isArchiveDataRequest && jsonRpcResp.Result != nil {
					logrus.Infof("Req for chain %d from %s %s(%v) 200 and cache it: %v", chainId, req.RemoteAddr,
						proxyRequest.data.Method, string(proxyRequest.reqBytes), string(btsResp))
					getCache().Add(string(reqKey), btsResp)
				}
			}

			if !proxyRequest.isArchiveDataRequest {
				logrus.Infof("Req for chain %d from %s %s(%v) 200", chainId, req.RemoteAddr, proxyRequest.data.Method,
					string(proxyRequest.reqBytes))
			}
		}
	}()

	btsResp, err = currentRunningConfig.Configs[chainId].Strategy.handle(proxyRequest)
	var isArchiveRequestText string
	if proxyRequest.isArchiveDataRequest {
		isArchiveRequestText = "(ArchiveData)"
		logrus.Debug("archive data call: ", string(proxyRequest.reqBytes))
	}

	if err != nil {
		w.WriteHeader(500)
		_, _ = w.Write(getErrorResponseBytes(proxyRequest.data.ID, err.Error()))
		logrus.Errorf("Req%s from %s %s(%v) 500 %s", isArchiveRequestText, req.RemoteAddr,
			proxyRequest.data.Method, string(proxyRequest.reqBytes), err.Error())
		return
	}

	_, _ = w.Write(btsResp)
}
