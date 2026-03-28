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

func (c *Client) ModelsSet(id string) (string, error) {
	res, err := c.sendRequest("models.set", ModelsSetParams{Model: id})
	if err != nil {
		return "", err
	}

	var payload struct {
		Previous string `json:"previous"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return "", err
	}
	return payload.Previous, nil
}

func (c *Client) Status(session string) (StatusPayload, error) {
	res, err := c.sendRequest("status", map[string]string{"session": session})
	if err != nil {
		return StatusPayload{}, err
	}

	var payload StatusPayload
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return StatusPayload{}, err
	}
	return payload, nil
}

func (c *Client) Compact(session string) (*CompactResult, error) {
	res, err := c.sendRequest("compact", map[string]string{"session": session})
	if err != nil {
		return nil, err
	}

	var payload CompactResult
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) Skills() ([]SkillInfo, error) {
	res, err := c.sendRequest("skills.list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Skills []SkillInfo `json:"skills"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Skills, nil
}

func (c *Client) MCPServers() ([]MCPServerInfo, error) {
	res, err := c.sendRequest("mcps.list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Servers []MCPServerInfo `json:"servers"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Servers, nil
}

func (c *Client) CronJobs() ([]CronJob, error) {
	res, err := c.sendRequest("cron.list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Jobs []CronJob `json:"jobs"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		return nil, err
	}
	return payload.Jobs, nil
}
