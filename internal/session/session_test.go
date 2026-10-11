// Place alongside session.go as internal/session/session_test.go.
package session

import (
	"context"
	"errors"
	"testing"

	"my-project/internal/agent"
)

func TestRunnerHistoryAndIsolation(t *testing.T) {
	var lengths []int
	r := NewRunner(func(_ context.Context, messages []agent.Message) (agent.Message, error) {
		lengths = append(lengths, len(messages))
		return agent.Message{Role: agent.RoleAssistant, Content: "ok"}, nil
	})
	ctx := context.Background()
	for _, id := range []string{"a", "a", "b"} {
		if _, err := r.Ask(ctx, id, "hello"); err != nil {
			t.Fatal(err)
		}
	}
	if len(lengths) != 3 || lengths[0] != 1 || lengths[1] != 3 || lengths[2] != 1 {
		t.Fatalf("unexpected histories: %v", lengths)
	}
}

func TestRunnerCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := NewRunner(func(context.Context, []agent.Message) (agent.Message, error) {
		t.Fatal("canceled request must not call agent")
		return agent.Message{}, nil
	})
	_, err := r.Ask(ctx, "a", "hello")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
