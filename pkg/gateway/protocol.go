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
	FrameRequestAlt FrameKind = "req"
	FrameResponseAlt FrameKind = "res"
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
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorFrame    `json:"error,omitempty"`
}

type LegacyResponseFrame struct {
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorFrame    `json:"error,omitempty"`
}

type EventFrame struct {
	EventName string          `json:"event"`
	Seq       int64           `json:"seq"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type DecodedFrame struct {
	Kind       FrameKind
	Request    RequestFrame
	Response   ResponseFrame
	Event      EventFrame
	IsLegacy  bool
}

type wireFrame struct {
	Type    FrameKind       `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorFrame     `json:"error,omitempty"`
	Name    string          `json:"name,omitempty"`
	Event   string          `json:"event,omitempty"`
	Seq     int64           `json:"seq,omitempty"`
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

func EncodeLegacyResponse(frame ResponseFrame) ([]byte, error) {
	return json.Marshal(wireFrame{
		Type:    FrameResponseAlt,
		ID:      frame.ID,
		OK:      frame.OK,
		Payload: frame.Result,
		Error:   frame.Error,
	})
}

func EncodeEvent(frame EventFrame) ([]byte, error) {
	data, err := json.Marshal(frame.Payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wireFrame{
		Type:  FrameEvent,
		Event: frame.EventName,
		Name:  string(data),
		Seq:   frame.Seq,
	})
}

func DecodeFrame(data []byte) (*DecodedFrame, error) {
	wire := wireFrame{}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}

	switch wire.Type {
	case FrameRequest, FrameRequestAlt:
		df := &DecodedFrame{Kind: FrameRequest, Request: RequestFrame{
			ID:     wire.ID,
			Method: wire.Method,
			Params: wire.Params,
		}}
		if wire.Type == FrameRequestAlt {
			df.IsLegacy = true
		}
		return df, nil
	case FrameResponse, FrameResponseAlt:
		rf := ResponseFrame{
			ID:    wire.ID,
			Error: wire.Error,
		}
		if wire.Result != nil {
			rf.Result = wire.Result
		} else if wire.Payload != nil {
			rf.Result = wire.Payload
		}
		rf.OK = wire.OK
		return &DecodedFrame{Kind: FrameResponse, Response: rf, IsLegacy: wire.Type == FrameResponseAlt}, nil
	case FrameEvent:
		return &DecodedFrame{Kind: FrameEvent, Event: EventFrame{
			EventName: wire.Event,
			Seq:       wire.Seq,
			Payload:   json.RawMessage(wire.Name),
		}}, nil
	default:
		return nil, fmt.Errorf("unknown frame type %q", wire.Type)
	}
}
