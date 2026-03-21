package client

import "encoding/json"

func (c *Client) ChatSend(session, message string) (string, error) {
	return c.SendAsync("chat.send", ChatSendParams{
		Session: session,
		Message: message,
	})
}

func (c *Client) ChatAbort(session, runID string) error {
	_, err := c.sendRequest("chat.abort", ChatAbortParams{
		Session: session,
		RunID:   runID,
	})
	return err
}

func (c *Client) ChatHistory(session string, limit int) ([]HistoryMessage, error) {
	res, err := c.sendRequest("chat.history", map[string]any{
		"session": session,
		"limit":   limit,
	})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Messages []HistoryMessage `json:"messages"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Messages, nil
}

func (c *Client) SessionsList() ([]SessionInfo, error) {
	res, err := c.sendRequest("sessions.list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Sessions []SessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Sessions, nil
}

func (c *Client) SessionsReset(key string) error {
	_, err := c.sendRequest("sessions.reset", map[string]string{"key": key})
	return err
}

func (c *Client) ModelsList() ([]ModelInfo, string, error) {
	res, err := c.sendRequest("models.list", map[string]string{})
	if err != nil {
		return nil, "", err
	}

	var payload struct {
		Models  []ModelInfo `json:"models"`
		Current string      `json:"current"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, "", err
	}
	return payload.Models, payload.Current, nil
}

func (c *Client) ModelsSet(id string) error {
	_, err := c.sendRequest("models.set", map[string]string{"id": id})
	return err
}

func (c *Client) Status() (json.RawMessage, error) {
	res, err := c.sendRequest("status", map[string]string{})
	if err != nil {
		return nil, err
	}
	return res.Payload, nil
}
