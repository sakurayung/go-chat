package transport

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/sakurayung/gochat/internal/chat"
)

func TestBadNameIsRefusedBeforeUpgrade(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	resp, err := http.Get(ts.URL + "/subscribe?name=" + url.QueryEscape("a b"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestDuplicateNameIsRejected(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	dialJoined(t, ts, "alice", "lobby")

	endpoint := "ws" + strings.TrimPrefix(ts.URL, "http") + "/subscribe?" + url.Values{
		"name": {"alice"},
		"room": {"lobby"},
	}.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	imposter, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer imposter.CloseNow()

	// The handshake already succeeded, so the reason travels as a message.
	var msg chat.Message
	if err := wsjson.Read(ctx, imposter, &msg); err != nil {
		t.Fatalf("read: %v", err)
	}
	if msg.Type != chat.MsgError || !strings.Contains(msg.Text, "already taken") {
		t.Fatalf("got %+v, want an error about a taken name", msg)
	}
	// The server wrote it, so the client did not get to pick From or Time.
	if msg.From != chat.ServerName || msg.Time.IsZero() {
		t.Fatalf("got %+v, want it written by the server itself", msg)
	}

	// The connection is then closed with a policy violation, which tells the
	// client that retrying will not help.
	code := websocket.CloseStatus(readUntilClosed(t, imposter))
	if code != websocket.StatusPolicyViolation {
		t.Fatalf("close code = %v, want %v", code, websocket.StatusPolicyViolation)
	}
}

func TestOversizedClientMessageClosesTheConnection(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	ws := dialJoined(t, ts, "alice", "lobby")

	// The read limit is a security boundary: a peer must not be able to make the server buffer as much as it likes.
	big := strings.Repeat("a", maxClientMessageLen*2)
	if err := wsjson.Write(context.Background(), ws, chat.Message{
		Type: chat.MsgChat,
		Text: big,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	code := websocket.CloseStatus(readUntilClosed(t, ws))
	if code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %v, want %v", code, websocket.StatusMessageTooBig)
	}
}

func TestUnsupportedMessageTypeIsRejected(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	ws := dialJoined(t, ts, "alice", "lobby")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if err := wsjson.Write(ctx, ws, chat.Message{Type: "let-me-in", Text: "hi"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The server says no and keeps the connection: only the bad message is
	// thrown away.
	got := waitFor(t, ws, chat.MsgError)
	// previously it was "unsupported message type" and it throwned an error
	// it must match to the capitalization in the subscribe.go file which is
	// "Unsupported message type"
	if !strings.Contains(got.Text, "Unsupported message type") {
		t.Fatalf("error = %+v", got)
	}
}

func TestLeaveIsBroadcast(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	alice := dialJoined(t, ts, "alice", "lobby")
	bob := dialJoined(t, ts, "bob", "lobby")

	// Leaving is simply closing the connection.
	if err := bob.Close(websocket.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("close bob: %v", err)
	}

	left := waitFor(t, alice, chat.MsgLeave)
	if left.From != "bob" {
		t.Fatalf("leave = %+v, want bob's", left)
	}
}
