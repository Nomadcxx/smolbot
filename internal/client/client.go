package client

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	url     string
	conn    *websocket.Conn
	mu      sync.Mutex
	writeMu sync.Mutex
	nextID  atomic.Int64
	pending sync.Map
	lastSeq int
	done    chan struct{}

	OnEvent func(Event)
	OnClose func()
}

func New(url string) *Client {
	c := &Client{url: url, done: make(chan struct{})}
	c.nextID.Store(0)
	return c
}

func (c *Client) SetOnEvent(fn func(Event)) {
	c.OnEvent = fn
}

func (c *Client) SetOnClose(fn func()) {
	c.OnClose = fn
}

func (c *Client) Connect() (*HelloPayload, error) {
	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.lastSeq = 0
	c.done = make(chan struct{})
	c.mu.Unlock()

	helloReq := Request{
		Type:   FrameReq,
		ID:     c.allocID(),
		Method: "hello",
		Params: HelloParams{
			Client:   "smolbot-tui",
			Version:  "0.1.0",
			Protocol: ProtocolVersion,
			Platform: runtime.GOOS,
		},
	}
	if err := conn.WriteJSON(helloReq); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write hello: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set deadline: %w", err)
	}
	_, raw, err := conn.ReadMessage()
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read hello: %w", err)
	}

	var res Response
	if err := json.Unmarshal(raw, &res); err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse hello: %w", err)
	}
	if !res.OK {
		conn.Close()
		if res.Error != nil {
			return nil, fmt.Errorf("hello: %s", res.Error.Message)
		}
		return nil, fmt.Errorf("hello rejected")
	}

	var payload HelloPayload
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		conn.Close()
		return nil, fmt.Errorf("hello payload: %w", err)
	}

	go c.readLoop()

	// Start ping goroutine to keep connection alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				c.mu.Lock()
				conn := c.conn
				c.mu.Unlock()
				if conn == nil {
					return
				}
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	return &payload, nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.lastSeq = 0
	if c.done != nil {
		close(c.done)
		c.done = nil
	}
}

func (c *Client) allocID() string {
	return fmt.Sprintf("%d", c.nextID.Add(1))
}

func (c *Client) sendRequest(method string, params any) (*Response, error) {
	id := c.allocID()
	ch := make(chan *Response, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	req := Request{
		Type:   FrameReq,
		ID:     id,
		Method: method,
		Params: params,
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	if err := c.writeJSON(conn, req); err != nil {
		return nil, err
	}

	select {
	case res := <-ch:
		if !res.OK {
			if res.Error != nil {
				return nil, fmt.Errorf("%s: %s", res.Error.Code, res.Error.Message)
			}
			return nil, fmt.Errorf("request failed")
		}
		return res, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response to %s", method)
	}
}

func (c *Client) SendAsync(method string, params any) (string, error) {
	res, err := c.sendRequest(method, params)
	if err != nil {
		return "", err
	}

	var payload ChatSendPayload
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return "", err
	}
	return payload.RunID, nil
}

func (c *Client) writeJSON(conn *websocket.Conn, payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(payload)
}

func (c *Client) readLoop() {
	defer func() {
		if c.OnClose != nil {
			c.OnClose()
		}
	}()

	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var frame Frame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}

		switch frame.Type {
		case FrameRes:
			var res Response
			if err := json.Unmarshal(raw, &res); err != nil {
				continue
			}
			if ch, ok := c.pending.Load(res.ID); ok {
				ch.(chan *Response) <- &res
			}
		case FrameEvent:
			var evt Event
			if err := json.Unmarshal(raw, &evt); err != nil {
				continue
			}
			if evt.Seq > 0 {
				if c.lastSeq > 0 && evt.Seq != c.lastSeq+1 {
					c.Close()
					return
				}
				c.lastSeq = evt.Seq
			}
			if c.OnEvent != nil {
				c.OnEvent(evt)
			}
		}
	}
}
