// Place this file at internal/session/session.go in the existing my-project module.
package session

import (
	"context"
	"errors"
	"sync"

	"my-project/internal/agent"
)

// ChatFunc is compatible with (*agent.Agent).Chat.
type ChatFunc func(context.Context, []agent.Message) (agent.Message, error)

type conversation struct {
	mu       sync.Mutex
	messages []agent.Message
}

type Runner struct {
	mu       sync.Mutex
	sessions map[string]*conversation
	chat     ChatFunc
}

func NewRunner(chat ChatFunc) *Runner {
	return &Runner{sessions: make(map[string]*conversation), chat: chat}
}

func (r *Runner) conversation(id string) *conversation {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.sessions[id]
	if !ok {
		c = &conversation{}
		r.sessions[id] = c
	}
	return c
}

// Ask stores only user + final assistant messages, not intermediate ToolCalls.
// Calls for the same session ID are serialized to avoid lost updates.
func (r *Runner) Ask(ctx context.Context, sessionID, question string) (agent.Message, error) {
	if r == nil || r.chat == nil {
		return agent.Message{}, errors.New("session runner is nil")
	}
	if sessionID == "" {
		return agent.Message{}, errors.New("sessionID is required")
	}
	if err := ctx.Err(); err != nil {
		return agent.Message{}, err
	}
	c := r.conversation(sessionID)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return agent.Message{}, err
	}
	input := append([]agent.Message(nil), c.messages...)
	input = append(input, agent.Message{Role: agent.RoleUser, Content: question})
	answer, err := r.chat(ctx, input)
	if err != nil {
		return agent.Message{}, err
	}
	if err := ctx.Err(); err != nil {
		return agent.Message{}, err
	}
	c.messages = append(input, answer)
	return answer, nil
}
