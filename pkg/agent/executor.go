package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

// toolExecResult holds the outcome of a single tool call execution.
type toolExecResult struct {
	message   provider.Message
	output    string
	errText   string
	delivered bool
	execErr   error
}

// ToolExecutor executes a batch of tool calls, running concurrent-safe tools
// in parallel when parallelEnabled is true.
type ToolExecutor struct {
	tools           *tool.Registry
	parallelEnabled bool
	req             Request
}

func newToolExecutor(tools *tool.Registry, parallelEnabled bool, req Request) *ToolExecutor {
	return &ToolExecutor{tools: tools, parallelEnabled: parallelEnabled, req: req}
}

// ExecuteAll executes all calls, emitting EventToolHint/Start/Done events via cb.
// Safe tools are run in parallel when parallelEnabled is true and len(calls) > 1.
// Unsafe tools always run sequentially after safe tools have completed.
// Returns the first tool execution error encountered (all other results are still returned).
func (e *ToolExecutor) ExecuteAll(
	ctx context.Context,
	calls []provider.ToolCall,
	tctx tool.ToolContext,
	cb EventCallback,
) ([]toolExecResult, error) {
	if !e.parallelEnabled || len(calls) <= 1 {
		return e.executeSequential(ctx, calls, tctx, cb)
	}

	// Partition calls into concurrent-safe and sequential groups.
	var safeCalls, unsafeCalls []provider.ToolCall
	for _, call := range calls {
		if e.tools.IsConcurrencySafe(call.Function.Name) {
			safeCalls = append(safeCalls, call)
		} else {
			unsafeCalls = append(unsafeCalls, call)
		}
	}

	// Only worth parallelising if there are at least 2 safe calls.
	if len(safeCalls) < 2 {
		return e.executeSequential(ctx, calls, tctx, cb)
	}

	var results []toolExecResult

	parallelResults, err := e.executeParallel(ctx, safeCalls, tctx, cb)
	results = append(results, parallelResults...)
	if err != nil {
		return results, err
	}

	if len(unsafeCalls) > 0 {
		seqResults, err := e.executeSequential(ctx, unsafeCalls, tctx, cb)
		results = append(results, seqResults...)
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// executeParallel runs all calls concurrently. It emits start events before
// launching goroutines and done events (in original order) after all complete.
func (e *ToolExecutor) executeParallel(
	ctx context.Context,
	calls []provider.ToolCall,
	tctx tool.ToolContext,
	cb EventCallback,
) ([]toolExecResult, error) {
	// Emit hint + start for all calls upfront so the UI can show them immediately.
	for _, call := range calls {
		emit(cb, Event{Type: EventToolHint, Content: call.Function.Name})
		emit(cb, Event{Type: EventToolStart, Content: call.Function.Name, Data: map[string]any{
			"input": call.Function.Arguments,
			"id":    call.ID,
		}})
	}

	results := make([]toolExecResult, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc provider.ToolCall) {
			defer wg.Done()
			result, err := e.tools.Execute(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments), tctx)
			results[idx] = e.buildResult(tc, result, err)
		}(i, call)
	}
	wg.Wait()

	// Emit done events in original order so the UI stays deterministic.
	for _, res := range results {
		emit(cb, Event{
			Type:    EventToolDone,
			Content: res.message.Name,
			Data: map[string]any{
				"deliveredToRequestTarget": res.delivered,
				"id":                       res.message.ToolCallID,
				"output":                   res.output,
				"error":                    res.errText,
			},
		})
	}

	// Surface the first hard error (all messages are already collected).
	for _, res := range results {
		if res.execErr != nil {
			return results, res.execErr
		}
	}
	return results, nil
}

// executeSequential runs calls one by one, mirroring the original agent loop behaviour.
// It returns immediately on the first tool execution error.
func (e *ToolExecutor) executeSequential(
	ctx context.Context,
	calls []provider.ToolCall,
	tctx tool.ToolContext,
	cb EventCallback,
) ([]toolExecResult, error) {
	results := make([]toolExecResult, 0, len(calls))
	for _, call := range calls {
		emit(cb, Event{Type: EventToolHint, Content: call.Function.Name})
		emit(cb, Event{Type: EventToolStart, Content: call.Function.Name, Data: map[string]any{
			"input": call.Function.Arguments,
			"id":    call.ID,
		}})

		result, err := e.tools.Execute(ctx, call.Function.Name, json.RawMessage(call.Function.Arguments), tctx)
		res := e.buildResult(call, result, err)
		results = append(results, res)

		emit(cb, Event{
			Type:    EventToolDone,
			Content: call.Function.Name,
			Data: map[string]any{
				"deliveredToRequestTarget": res.delivered,
				"id":                       call.ID,
				"output":                   res.output,
				"error":                    res.errText,
			},
		})

		if err != nil {
			return results, err
		}
	}
	return results, nil
}

// buildResult converts a raw tool.Execute response into a toolExecResult.
func (e *ToolExecutor) buildResult(tc provider.ToolCall, result *tool.Result, execErr error) toolExecResult {
	if execErr != nil {
		return toolExecResult{
			execErr: execErr,
			message: provider.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("error: %v", execErr),
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			},
		}
	}

	output := truncateString(firstNonEmptyString(result.Output, result.Content), 16000)
	errText := truncateString(result.Error, 16000)
	content := firstNonEmptyString(output, errText)

	delivered := false
	if tc.Function.Name == "message" && result != nil {
		delivered = sameTargetDelivery(e.req, result.Metadata)
	}

	return toolExecResult{
		output:    output,
		errText:   errText,
		delivered: delivered,
		message: provider.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: tc.ID,
			Name:       tc.Function.Name,
		},
	}
}
