package wshandler

import "encoding/json"

type Message struct {
	Action    string          `json:"action"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type Response struct {
	Action    string   `json:"action"`
	RequestID string   `json:"request_id,omitempty"`
	Success   bool     `json:"success"`
	Payload   any      `json:"payload,omitempty"`
	Error     *WSError `json:"error,omitempty"`
}

type WSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
