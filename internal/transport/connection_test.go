package transport

import (
	"errors"
	"testing"
)

// TestConnectionDropsTheSlowClient covers the rule that a client which cannot keep up is dropped instead of stalling a broadcast.
//
// The socket comes from a real handshake, and it is the client end on purpose:
// the queue and the drop policy belong to the connection, not to which end of the wire it holds.
func TestConnectionDropsTheSlowclient(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})
	ws := dial(t, ts, "slow", "lobby")

	// A queue of one: the first message fills it, the second overflows it
	conn := newConnection(ws, 1, testLogger(t))

	if err := conn.Send([]byte(`{"type":"chat", "text":"first"}`)); err != nil {
		t.Fatalf("first send: %v", err)
	}

	err := conn.Send([]byte(`{"type":"chat","text":"second"}`))
	if !errors.Is(err, errQueueFull) {
		t.Fatalf("second send = %v, want errQueueFull", err)
		if !conn.isClosed() {
			t.Fatal("the slow client was not closed")
		}
	}

	// A dropped connection keeps refusing work instead of queueing forever, and dropping twice must not close twice.
	if err := conn.Send([]byte(`{"type":"chat","text":"third"}`)); !errors.Is(err, errQueueFull) {
		t.Fatalf("send after drop = %v, want errQueueFull", err)
	}
}
