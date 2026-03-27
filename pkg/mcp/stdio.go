package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc

	writeMu sync.Mutex
	stateMu sync.Mutex
	nextID  atomic.Int64
	pending map[string]chan responseResult
	closed  bool
	readErr error
}

type responseResult struct {
	resp jsonRPCResponse
	err  error
}

var errTransportWriteInterrupted = errors.New("mcp transport write interrupted")

type writeResult struct {
	err     error
	aborted bool
}

func NewStdioTransport(ctx context.Context, command string, args []string, env map[string]string) (*StdioTransport, error) {
	procCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procCtx, command, args...)

	cmdEnv := cmd.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start mcp server %q: %w", command, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	t := &StdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		cancel:  cancel,
		pending: make(map[string]chan responseResult),
	}
	t.startReadLoop(scanner)
	return t, nil
}

func (t *StdioTransport) Send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := newJSONRPCIntID(t.nextID.Add(1))
	return t.sendRequest(ctx, &id, method, params)
}

func (t *StdioTransport) Notify(ctx context.Context, method string, params any) error {
	_, err := t.sendRequest(ctx, nil, method, params)
	return err
}

func (t *StdioTransport) sendRequest(ctx context.Context, id *jsonRPCID, method string, params any) (json.RawMessage, error) {
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		rawParams = b
	}

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	waitCh := make(chan responseResult, 1)
	if id != nil {
		if err := t.registerPending(string(*id), waitCh); err != nil {
			return nil, err
		}
		defer t.unregisterPending(string(*id), waitCh)
	}

	if err := t.write(ctx, append(data, '\n')); err != nil {
		return nil, err
	}
	if id == nil {
		return nil, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res, ok := <-waitCh:
			if !ok {
				return nil, fmt.Errorf("mcp server closed stdout")
			}
			if res.err != nil {
				return nil, fmt.Errorf("read from mcp server: %w", res.err)
			}
			if res.resp.Error != nil {
				return nil, res.resp.Error
			}
			return res.resp.Result, nil
		}
	}
}

func (t *StdioTransport) write(ctx context.Context, data []byte) error {
	abort := make(chan struct{})
	acquired := make(chan struct{})
	started := make(chan struct{})
	done := make(chan writeResult, 1)
	go func() {
		t.writeMu.Lock()
		defer t.writeMu.Unlock()
		close(acquired)
		select {
		case <-abort:
			done <- writeResult{aborted: true}
			return
		default:
		}
		close(started)
		_, err := t.stdin.Write(data)
		done <- writeResult{err: err}
	}()

	select {
	case <-acquired:
	case <-ctx.Done():
		close(abort)
		return ctx.Err()
	}

	select {
	case result := <-done:
		if result.aborted {
			return ctx.Err()
		}
		if result.err != nil {
			return fmt.Errorf("write to mcp server: %w", result.err)
		}
		return nil
	case <-ctx.Done():
		close(abort)
		select {
		case <-started:
			if t.cancel != nil {
				t.cancel()
			}
			return fmt.Errorf("%w: %w", errTransportWriteInterrupted, ctx.Err())
		default:
			return ctx.Err()
		}
	}
}

func (t *StdioTransport) Close() error {
	if t == nil {
		return nil
	}
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.cmd == nil {
		if t.cancel != nil {
			t.cancel()
		}
		return nil
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- t.cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if t.cancel != nil {
			t.cancel()
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.Success() {
			return nil
		}
		return err
	case <-time.After(500 * time.Millisecond):
		if t.cancel != nil {
			t.cancel()
		}
		select {
		case err := <-waitCh:
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.Success() {
				return nil
			}
			return err
		case <-time.After(500 * time.Millisecond):
			return fmt.Errorf("timed out waiting for mcp server process to exit")
		}
	}
}

func (t *StdioTransport) registerPending(id string, ch chan responseResult) error {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.closed {
		if t.readErr != nil {
			return fmt.Errorf("read from mcp server: %w", t.readErr)
		}
		return fmt.Errorf("mcp server closed stdout")
	}
	t.pending[id] = ch
	return nil
}

func (t *StdioTransport) unregisterPending(id string, ch chan responseResult) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if current, ok := t.pending[id]; ok && current == ch {
		delete(t.pending, id)
	}
}

func (t *StdioTransport) startReadLoop(scanner *bufio.Scanner) error {
	go func() {
		for scanner.Scan() {
			var resp jsonRPCResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				continue
			}
			if resp.ID == nil {
				continue
			}

			t.stateMu.Lock()
			ch, ok := t.pending[string(*resp.ID)]
			if ok {
				delete(t.pending, string(*resp.ID))
			}
			t.stateMu.Unlock()
			if ok {
				ch <- responseResult{resp: resp}
				close(ch)
			}
		}
		err := scanner.Err()
		if err == nil {
			err = io.EOF
		}
		t.stateMu.Lock()
		t.closed = true
		t.readErr = err
		pending := t.pending
		t.pending = make(map[string]chan responseResult)
		t.stateMu.Unlock()
		for _, ch := range pending {
			ch <- responseResult{err: err}
			close(ch)
		}
	}()
	return nil
}
