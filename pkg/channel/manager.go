package channel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

type Manager struct {
	mu             sync.RWMutex
	channels       map[string]Channel
	inboundHandler Handler
	running        map[Channel]bool
}

func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		running:  make(map[Channel]bool),
	}
}

func (m *Manager) Register(channel Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[channel.Name()] = channel
}

func (m *Manager) SetInboundHandler(handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inboundHandler = handler
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.inboundHandler == nil {
		m.mu.Unlock()
		return errors.New("channel manager: SetInboundHandler must be called before Start")
	}
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Name() < channels[j].Name() })
	m.mu.Unlock()

	for _, channel := range channels {
		if err := channel.Start(ctx, m.inboundHandler); err != nil {
			log.Printf("[channel] %s failed to start (running without it): %v", channel.Name(), err)
			continue
		}
		m.mu.Lock()
		m.running[channel] = true
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	toStop := make([]Channel, 0, len(m.running))
	for ch := range m.running {
		toStop = append(toStop, ch)
	}
	m.running = make(map[Channel]bool)
	m.mu.Unlock()

	for _, ch := range toStop {
		if err := ch.Stop(ctx); err != nil {
			return fmt.Errorf("stop channel %s: %w", ch.Name(), err)
		}
	}
	return nil
}

func (m *Manager) Route(ctx context.Context, channelName, chatID, content string) error {
	m.mu.RLock()
	channel, ok := m.channels[channelName]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("channel %q not registered", channelName)
	}
	return channel.Send(ctx, OutboundMessage{
		Channel: channelName,
		ChatID:  chatID,
		Content: content,
	})
}

func (m *Manager) Statuses(ctx context.Context) map[string]Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]Status, len(m.channels))
	for name, channel := range m.channels {
		status := Status{State: "registered"}
		if reporter, ok := channel.(StatusReporter); ok {
			reported, err := reporter.Status(ctx)
			if err != nil {
				status = Status{State: "error", Detail: err.Error()}
			} else if reported.State != "" {
				status = reported
			}
		}
		statuses[name] = status
	}
	return statuses
}

func (m *Manager) Login(ctx context.Context, channelName string) error {
	return m.LoginWithUpdates(ctx, channelName, nil)
}

func (m *Manager) LoginWithUpdates(ctx context.Context, channelName string, report func(Status) error) error {
	m.mu.RLock()
	channel, ok := m.channels[channelName]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("channel %q not registered", channelName)
	}
	if handler, ok := channel.(InteractiveLoginHandler); ok {
		return handler.LoginWithUpdates(ctx, report)
	}
	handler, ok := channel.(LoginHandler)
	if !ok {
		return fmt.Errorf("channel %q does not support login", channelName)
	}
	if err := handler.Login(ctx); err != nil {
		return err
	}
	if report != nil {
		if reporter, ok := channel.(StatusReporter); ok {
			status, err := reporter.Status(ctx)
			if err != nil {
				return err
			}
			if status.State != "" {
				if err := report(status); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Manager) ChannelNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *Manager) Watch(ctx context.Context, interval time.Duration, onDead func(name string, status Status)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for name, status := range m.Statuses(ctx) {
				if status.State != "connected" {
					onDead(name, status)
				}
			}
		}
	}
}
