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
		RunID: runID,
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

	var payload []HistoryMessage
	if err := json.Unmarshal(res.Result, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) SessionsList() ([]SessionInfo, error) {
	res, err := c.sendRequest("sessions.list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var payload []SessionInfo
	if err := json.Unmarshal(res.Result, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) SessionsReset(key string) error {
	_, err := c.sendRequest("sessions.reset", map[string]string{"session": key})
	return err
}

func (c *Client) ModelsList() ([]ModelInfo, string, error) {
	res, err := c.sendRequest("models.list", map[string]string{})
	if err != nil {
		return nil, "", err
	}

	var payload []ModelInfo
	if err := json.Unmarshal(res.Result, &payload); err != nil {
		return nil, "", err
	}
	current := ""
	if len(payload) > 0 {
		current = payload[0].ID
	}
	return payload, current, nil
}

func (c *Client) ModelsSet(id string) error {
	_, err := c.sendRequest("models.set", map[string]string{"model": id})
	return err
}

func (c *Client) Status() (StatusPayload, error) {
	res, err := c.sendRequest("status", map[string]string{})
	if err != nil {
		return StatusPayload{}, err
	}
	var payload StatusPayload
	if err := json.Unmarshal(res.Result, &payload); err != nil {
		return StatusPayload{}, err
	}
	return payload, nil
}
