package transport

import (
	"net/http"
	"testing"

	"github.com/coder/websocket"
)

func TestDisconnectAllClosesClients(t *testing.T) {
	ts, srv, _ := newTestServer(t, Config{})

	ws := dialJoined(t, ts, "alice", "lobby")

	// What Ctrl-C does: every client is told the server is going away instead of waiting for a TCP timeout.
	srv.DisconnectAll()

	code := websocket.CloseStatus(readUntilClosed(t, ws))
	if code != websocket.StatusGoingAway {
		t.Fatalf("close code = %v, want %v", code, websocket.StatusGoingAway)
	}
}

func TestServerWithoutAssetsAnswersNotFound(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	// The transport does not care where the front end comes from, and with no
	// Assets it must not fall back to serving the source tree.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
