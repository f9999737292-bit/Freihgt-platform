package push

import (
	"context"
	"fmt"
	"sync"
)

type FakeProvider struct {
	mu sync.Mutex

	AvailableFlag bool
	SendFn        func(ctx context.Context, msg Message) (SendResult, error)
	Sent          []Message
}

func NewFakeProvider() *FakeProvider {
	return &FakeProvider{AvailableFlag: true}
}

func (p *FakeProvider) Name() string { return "fake" }

func (p *FakeProvider) Available() bool { return p.AvailableFlag }

func (p *FakeProvider) Send(ctx context.Context, msg Message) (SendResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.SendFn != nil {
		return p.SendFn(ctx, msg)
	}
	if !p.AvailableFlag {
		return SendResult{}, ErrProviderUnavailable
	}
	p.Sent = append(p.Sent, msg)
	return SendResult{ProviderMessageID: fmt.Sprintf("fake-%s", msg.TaskID)}, nil
}

func (p *FakeProvider) SentCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.Sent)
}
