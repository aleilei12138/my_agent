package streamhttp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSESequence(t *testing.T) {
	handler := Handler(func(_ context.Context, emit func(string) error) error {
		if err := emit("你"); err != nil {
			return err
		}
		return emit("好")
	})
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/", nil))
	got := w.Body.String()
	if !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") ||
		strings.Count(got, "event: delta") != 2 ||
		!strings.Contains(got, "event: done") {
		t.Fatalf("invalid SSE response: headers=%v body=%q", w.Header(), got)
	}
}
