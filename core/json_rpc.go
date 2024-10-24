package core

type RequestData struct {
	JsonRpc string        `json:"jsonrpc"`
	ID      int64         `json:"id,omitempty"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type JsonRpcResponse struct {
	JsonRpc string       `json:"jsonrpc"`
	ID      int64        `json:"id"`
	Err     JsonRpcError `json:"error"`
	Result  interface{}  `json:"result"`
}

type JsonRpcError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}
