package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// the handle function will execute concurrently
type Upstream interface {
	handle(*Request) ([]byte, error)
	updateBlockNumber()
	getRpcUrl() string
	isAlive() bool
	getLatancy() int64
}

type wsProxyRequest struct {
	*Request
	id       int64
	resBytes chan []byte
}

type wsProxyResponse struct {
	ID int64 `json:"id"`
}

type WsUpstream struct {
	chainId      uint64
	url          string
	requestQueue chan *wsProxyRequest
	nextID       int64     // proxy request id
	requests     *sync.Map // proxy request id => proxy request
	blockNumber  int
	latency      int64
}

type HttpUpstream struct {
	ctx         context.Context
	chainId     uint64
	url         string
	oldTrieUrl  string
	blockNumber int
	latency     int64
}

type BlockNumberResponseData struct {
	JsonRpc string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Result  string `json:"result"`
}

func newUpstream(ctx context.Context, chainId uint64, urlString string, oldTrieUrlString string) Upstream {
	u, err := url.Parse(urlString)

	if err != nil {
		panic(err)
	}

	ou := u

	if urlString != oldTrieUrlString {
		ou, err = url.Parse(oldTrieUrlString)

		if err != nil {
			panic(err)
		}
	}

	var up Upstream

	if u.Scheme == "http" || u.Scheme == "https" {
		up = newHttpUpstream(ctx, chainId, u, ou)
	} else if u.Scheme == "ws" || u.Scheme == "wss" {
		up = newWsStream(ctx, chainId, u)
	} else {
		panic(fmt.Errorf("unsuportted url schema %s", u.Scheme))
	}

	return up
}

func (u *HttpUpstream) handle(request *Request) ([]byte, error) {
	logrus.Infof("%v handled by %v", request.data.Method, u.url)

	ul := u.url

	if request.isOldTrieRequest(u.blockNumber) {
		ul = u.oldTrieUrl
	}

	upstreamReq, _ := http.NewRequest("POST", ul, bytes.NewReader(request.reqBytes))
	upstreamReq.Header.Set("Content-Type", "application/json")

	res, err := httpClient.Do(upstreamReq)

	if err != nil {
		logrus.Errorf("http upstream client do request error: %+v", err)
		return nil, err
	}

	bts, err := ioutil.ReadAll(res.Body)

	if err != nil {
		logrus.Errorf("http upstream io readall error: %+v", err)
		return nil, err
	}

	return bts, nil
}

func (u *HttpUpstream) updateBlockNumber() {
	req := getBlockNumberRequest(u.chainId)
	var latency int64
	startTime := time.Now()
	bts, err := u.handle(req)
	endTime := time.Now()
	if err != nil {
		latency = math.MaxInt64
	} else {
		latency = int64(endTime.Sub(startTime))
	}
	var res BlockNumberResponseData
	_ = json.Unmarshal(bts, &res)

	blockNumber, _ := strconv.ParseInt(res.Result, 0, 64)
	u.blockNumber = int(blockNumber)
	u.latency = latency
}

func (u *HttpUpstream) isAlive() bool {
	return u.latency != math.MaxInt64
}

func (u *HttpUpstream) getLatancy() int64 {
	return u.latency
}

func (u *HttpUpstream) getRpcUrl() string {
	return u.url
}

func (u *WsUpstream) handle(request *Request) ([]byte, error) {
	logrus.Infof("%v handled by %v", request.data.Method, u.url)

	proxyRequest := &wsProxyRequest{
		request,
		atomic.AddInt64(&u.nextID, 1),
		make(chan []byte),
	}

	u.requests.Store(proxyRequest.id, proxyRequest)
	defer u.requests.Delete(proxyRequest.id)

	select {
	case u.requestQueue <- proxyRequest:
	case <-time.After(5 * time.Second): // TODO use a configurable timeout
		return nil, TimeoutError
	}

	select {
	case res := <-proxyRequest.resBytes:
		return res, nil
	case <-time.After(5 * time.Second): // TODO use a configurable timeout
		return nil, TimeoutError
	}
}

func (u *WsUpstream) updateBlockNumber() {
	req := getBlockNumberRequest(u.chainId)
	var latency int64
	startTime := time.Now()
	bts, err := u.handle(req)
	endTime := time.Now()
	if err != nil {
		latency = math.MaxInt64
	} else {
		latency = int64(endTime.Sub(startTime))
	}
	var res BlockNumberResponseData
	_ = json.Unmarshal(bts, &res)

	blockNumber, _ := strconv.ParseInt(res.Result, 0, 64)
	u.blockNumber = int(blockNumber)
	u.latency = latency
}

func (u *WsUpstream) isAlive() bool {
	return u.latency != math.MaxInt64
}

func (u *WsUpstream) getLatancy() int64 {
	return u.latency
}

func (u *WsUpstream) getRpcUrl() string {
	return u.url
}

func (u *WsUpstream) run(ctx context.Context) {
	logrus.Debugf("ws %s run", u.url)
	defer logrus.Debugf("ws %s run exit", u.url)

	for {
		conn, _, err := websocket.DefaultDialer.Dial(u.url, nil)

		if err != nil {
			seconds := 5 // TODO configurable
			logrus.Errorf("ws upstream %s %v, will retry after %d seconds", u.url, err, seconds)

			select {
			case <-ctx.Done():
				// global stop
				return
			case <-time.After(time.Second * time.Duration(seconds)):
				continue
			}

		}

		logrus.Infof("ws upstream %s connected", u.url)
		u.runConn(ctx, conn)

		select {
		case <-ctx.Done():
			// global stop
			return
		}
	}
}

// return the connection context
func (u *WsUpstream) runConn(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()

	// connContext is for current connection
	// any error occurs, the context will be cancelled
	connContext, done := context.WithCancel(ctx)

	// request loop
	go func() {
		logrus.Debugf("conn request loop start")
		defer logrus.Debugf("conn request loop stop")
		defer done()
		for {
			select {
			case <-connContext.Done():
				// if the conn is invalid, exit
				return
			case wsProxyRequest := <-u.requestQueue:
				// use proxy ID
				wsProxyRequest.Request.data.ID = wsProxyRequest.id

				bts, _ := json.Marshal(wsProxyRequest.Request.data)
				err := conn.WriteMessage(websocket.TextMessage, bts)

				if err != nil {
					logrus.Errorf("write request to upstream failed %v", err)
					return
				}
			}

		}
	}()

	// response loop
	go func() {
		logrus.Debugf("conn response loop start")
		defer logrus.Debugf("conn response loop stop")
		defer done()

		for {
			t, p, err := conn.ReadMessage()

			if err != nil {
				logrus.Errorf("read response from upstream failed %v", err)
				break
			}

			if t != websocket.TextMessage {
				logrus.Infof("not a text message %v", p)
				continue
			}

			var res wsProxyResponse
			_ = json.Unmarshal(p, &res)

			if r, exist := u.requests.Load(res.ID); exist {
				if req, ok := r.(*wsProxyRequest); ok {
					req.resBytes <- p
				}
			}
		}
	}()

	<-connContext.Done()
}

func newHttpUpstream(ctx context.Context, chainId uint64, url *url.URL, oldTrieUrl *url.URL) *HttpUpstream {
	up := &HttpUpstream{
		ctx:        ctx,
		chainId:    chainId,
		url:        url.String(),
		oldTrieUrl: oldTrieUrl.String(),
	}

	if url != oldTrieUrl {
		setBlockNumber := func() {
			req := getBlockNumberRequest(chainId)
			bts, _ := up.handle(req)

			var res BlockNumberResponseData
			_ = json.Unmarshal(bts, &res)

			blockNumber, _ := strconv.ParseInt(res.Result, 0, 64)
			up.blockNumber = int(blockNumber)

		}

		go func() {
			time.Sleep(5 * time.Second)
			setBlockNumber()
		}()

		logrus.Infof("start old trie http upstream, blockNumber: %d", up.blockNumber)

		go func() {
			for {
				setBlockNumber()
				time.Sleep(30 * time.Second)
			}
		}()
	}

	return up
}

func newWsStream(ctx context.Context, chainId uint64, url *url.URL) *WsUpstream {
	upstream := &WsUpstream{
		chainId:      chainId,
		url:          url.String(),
		requestQueue: make(chan *wsProxyRequest),
		nextID:       time.Now().Unix(),
		requests:     &sync.Map{},
	}

	logrus.Infof("new upstream %s", url)
	go upstream.run(ctx)

	return upstream
}
