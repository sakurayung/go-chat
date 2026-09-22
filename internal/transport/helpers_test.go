package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/sakurayung/gochat/internal/chat"
)

// testTimeout bounds every read so a bug shows up as a failed test instead of a hanging one
const testTimeout = 5 * time.Second

// syncWriter is an io.Writer that is safe for concurrent use.
//
// The server logs from connection goroutines, which can outlive the test body
// so a logger must never write to *testing.T directly.
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// testLogger collects server logs and writes them out when the test ends.
// The dump happens in a cleanup, where talking to *testing.T is safe again.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()

	w := &syncWriter{}
	t.Cleanup(func() {
		for line := range strings.SplitSeq(strings.TrimSpace(w.String()), "\n") {
			if line != "" {
				t.Logf("server: %s", line)
			}
		}
	})
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// newTestServer starts a real HTTP server around a Server, so the tests take the
// same path a browser does: a real handshake over a real socket.
func newTestServer(t *testing.T, cfg Config) (*httptest.Server, *Server, *chat.Registry) {
	t.Helper()

	if cfg.Logger == nil {
		cfg.Logger = testLogger(t)
	}
	if cfg.Rooms == nil {
		cfg.Rooms = chat.NewRegistry()
	}

	srv := NewServer(cfg)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	return ts, srv, cfg.Rooms
}

// dial joins a room the way the browser does. An empty room leaves the query parameter off,
// which is what the default room is for.
func dial(t *testing.T, ts *httptest.Server, name, room string) *websocket.Conn {
	t.Helper()

	query := url.Values{
		"name": {name},
	}
	if room != "" {
		query.Set("room", room)
	}
	endpoint := "ws" + strings.TrimPrefix(ts.URL, "http") + "/subscribe?" + query.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatalf("dial as %q: %v", name, err)
	}
	t.Cleanup(func() { ws.CloseNow() })

	return ws
}

// dialJoined dials and then waits for the presence snapshot, which the server
// sends only once the client is really in the room. Tests that look at server
// state go through this so they do not race the join.
func dialJoined(t *testing.T, ts *httptest.Server, name, room string) *websocket.Conn {
	t.Helper()

	ws := dial(t, ts, name, room)
	waitFor(t, ws, chat.MsgPresence)
	return ws
}

func readMessage(t *testing.T, ws *websocket.Conn) chat.Message {
	t.Helper()

	// this pattern is called cancel to release resources
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var msg chat.Message
	if err := wsjson.Read(ctx, ws, &msg); err != nil {
		t.Fatalf("read: %v", err)
	}
	return msg
}

// waitFor reads and discards messages until one of the wanted type shows up.
func waitFor(t *testing.T, ws *websocket.Conn, want chat.MessageType) chat.Message {
	t.Helper()

	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		if msg := readMessage(t, ws); msg.Type == want {
			return msg
		}
	}
	t.Fatalf("no %q message within %s", want, testTimeout)
	return chat.Message{}
}

func sendChat(t *testing.T, ws *websocket.Conn, text string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if err := wsjson.Write(ctx, ws, chat.Message{
		Type: chat.MsgChat,
		Text: text,
	}); err != nil {
		t.Fatalf("send chat: %v", err)
	}
}

// readUntilClosed drains a connection until it fails and returns that error.
func readUntilClosed(t *testing.T, ws *websocket.Conn) error {
	t.Helper()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		_, _, err := ws.Read(ctx)
		cancel()
		if err != nil {
			t.Logf("readUntilClosed: %T %v", err, err)
			return err
		}
	}
}

// roomList reads GET /rooms.
func roomList(t *testing.T, ts *httptest.Server) []rooomJSON {
	t.Helper()

	resp, err := http.Get(ts.URL + "/rooms")
	if err != nil {
		t.Fatalf("get /rooms: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var views []rooomJSON
	if err := json.NewDecoder(resp.Body).Decode(&views); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return views
}
