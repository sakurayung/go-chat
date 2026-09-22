package transport

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/sakurayung/gochat/internal/chat"
)

func TestRoomsEndpoint(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	dialJoined(t, ts, "alice", "lobby")
	dialJoined(t, ts, "bob", "lobby")
	dialJoined(t, ts, "carol", "games")

	views := roomList(t, ts)
	if len(views) != 2 {
		t.Fatalf("views = %+v, want 2 rooms", views)
	}

	if views[0].Name != "games" || strings.Join(views[0].Members, ",") != "carol" {
		t.Fatalf("views[0] = %+v", views[0])
	}
	if views[1].Name != "lobby" || strings.Join(views[1].Members, ",") != "alice,bob" {
		t.Fatalf("views[1] = %+v", views[1])
	}
}

func TestWrongMethodFallsThroughToAssets(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	resp, err := http.Post(ts.URL+"/rooms", "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestEmptyRoomIsForgotten(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	ws := dialJoined(t, ts, "alice", "games")
	if err := ws.Close(websocket.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("close: %v", err)
	}

	deadline := time.Now().Add(testTimeout)
	for {
		if views := roomList(t, ts); len(views) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("rooms = %+v, want an empty list", roomList(t, ts))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func TestAnnounceReachesClients(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	alice := dialJoined(t, ts, "alice", "lobby")

	resp, err := http.Post(ts.URL+"/announce", "text/plain", strings.NewReader("maintenance in 5 minutes"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	msg := waitFor(t, alice, chat.MsgSystem)
	if msg.From != chat.ServerName || msg.Text != "maintenance in 5 minutes" {
		t.Fatalf("system message = %+v", msg)
	}
}

func TestAnnounceIsRateLimited(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	// The limiter allows a burst of 8, so the 9th immediate call has to be
	// refused. Waiting would have blocked instead, which is wrong for an HTTP
	// endpoint: the caller should be told to come back later.
	var status int
	for range 9 {
		resp, err := http.Post(ts.URL+"/announce", "text/plain", strings.NewReader("announce"))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		status = resp.StatusCode
		resp.Body.Close()
	}

	if status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestAnnounceRejectsAnOversizedBody(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	body := strings.Repeat("a", chat.MaxMessageLen+1)
	resp, err := http.Post(ts.URL+"/announce", "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}
