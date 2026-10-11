// Place in a standalone package, e.g. internal/streamhttp/sse.go.
// This is an HTTP adapter; pass a real Provider's streaming function to Handler.
package streamhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type StreamFunc func(ctx context.Context, emit func(delta string) error) error

func Handler(stream StreamFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if stream == nil {
			http.Error(w, "stream not configured", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		controller := http.NewResponseController(w)
		send := func(event string, payload any) error {
			data, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
				return err
			}
			return controller.Flush()
		}
		err := stream(r.Context(), func(delta string) error {
			return send("delta", map[string]string{"text": delta})
		})
		if err != nil {
			_ = send("error", map[string]string{"message": "stream interrupted"})
			return
		}
		_ = send("done", map[string]string{"status": "ok"})
	}
}
