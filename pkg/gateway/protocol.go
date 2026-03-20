package gateway

import (
	"encoding/json"
	"fmt"
)

type FrameKind string

const (
	FrameEvent    FrameKind = "event"
	FrameRequest  FrameKind = "request"
	FrameResponse FrameKind = "response"
)

type RequestFrame struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type ErrorFrame struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ResponseFrame struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorFrame     `json:"error,omitempty"`
}

type EventFrame struct {
	Name  string          `json:"name"`
	Seq   int64           `json:"seq"`
	Event json.RawMessage `json:"event,omitempty"`
}

type DecodedFrame struct {
	Kind     FrameKind
	Request  RequestFrame
	Response ResponseFrame
	Event    EventFrame
}

type wireFrame struct {
	Type   FrameKind       `json:"type"`
	ID     string          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorFrame     `json:"error,omitempty"`
	Name   string          `json:"name,omitempty"`
	Seq    int64           `json:"seq,omitempty"`
	Event  json.RawMessage `json:"event,omitempty"`
}

func EncodeRequest(frame RequestFrame) ([]byte, error) {
	return json.Marshal(wireFrame{
		Type:   FrameRequest,
		ID:     frame.ID,
		Method: frame.Method,
		Params: frame.Params,
	})
}

func EncodeResponse(frame ResponseFrame) ([]byte, error) {
	return json.Marshal(wireFrame{
		Type:   FrameResponse,
		ID:     frame.ID,
		Result: frame.Result,
		Error:  frame.Error,
	})
}

func EncodeError(frame ResponseFrame) ([]byte, error) {
	return EncodeResponse(frame)
}

func EncodeEvent(frame EventFrame) ([]byte, error) {
	return json.Marshal(wireFrame{
		Type:  FrameEvent,
		Name:  frame.Name,
		Seq:   frame.Seq,
		Event: frame.Event,
	})
}

func DecodeFrame(data []byte) (*DecodedFrame, error) {
	wire := wireFrame{}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}

	switch wire.Type {
	case FrameRequest:
		return &DecodedFrame{Kind: FrameRequest, Request: RequestFrame{
			ID:     wire.ID,
			Method: wire.Method,
			Params: wire.Params,
		}}, nil
	case FrameResponse:
		return &DecodedFrame{Kind: FrameResponse, Response: ResponseFrame{
			ID:     wire.ID,
			Result: wire.Result,
			Error:  wire.Error,
		}}, nil
	case FrameEvent:
		return &DecodedFrame{Kind: FrameEvent, Event: EventFrame{
			Name:  wire.Name,
			Seq:   wire.Seq,
			Event: wire.Event,
		}}, nil
	default:
		return nil, fmt.Errorf("unknown frame type %q", wire.Type)
	}
}
