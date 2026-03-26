package provider

import (
	"context"
	"io"
	"sort"
)

type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*Response, error)
	ChatStream(ctx context.Context, req ChatRequest) (*Stream, error)
	Name() string
}

type Stream struct {
	recvFn  func() (*StreamDelta, error)
	closeFn func() error
}

func NewStream(recvFn func() (*StreamDelta, error), closeFn func() error) *Stream {
	return &Stream{recvFn: recvFn, closeFn: closeFn}
}

func (s *Stream) Recv() (*StreamDelta, error) {
	if s == nil || s.recvFn == nil {
		return nil, io.EOF
	}
	return s.recvFn()
}

func (s *Stream) Close() error {
	if s == nil || s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}

func AccumulateStream(stream *Stream) (*Response, error) {
	defer stream.Close()

	resp := &Response{}
	toolCalls := map[int]*ToolCall{}
	order := map[int]struct{}{}

	for {
		delta, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if delta == nil {
			continue
		}

		resp.Content += delta.Content
		resp.ReasoningContent += delta.ReasoningContent

		for _, toolCall := range delta.ToolCalls {
			idx := toolCall.Index
			existing, ok := toolCalls[idx]
			if !ok {
				copyCall := toolCall
				toolCalls[idx] = &copyCall
				order[idx] = struct{}{}
				continue
			}

			if toolCall.ID != "" {
				existing.ID = toolCall.ID
			}
			if toolCall.Function.Name != "" {
				existing.Function.Name = toolCall.Function.Name
			}
			existing.Function.Arguments += toolCall.Function.Arguments
		}

		if delta.Usage != nil {
			resp.Usage = *delta.Usage
		}

		if delta.FinishReason != nil {
			resp.FinishReason = *delta.FinishReason
		}
	}

	indexes := make([]int, 0, len(order))
	for idx := range order {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	for _, idx := range indexes {
		resp.ToolCalls = append(resp.ToolCalls, *toolCalls[idx])
	}

	return resp, nil
}
