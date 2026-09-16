package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// fakeSender records what a client was sent and can be told to fail.
//
// This is what the Sender interface buys: the room can be tested end to end
// without a socket, a server or a goroutine. It is guarded by a mutex because
// one of the tests drives the room from several goroutines.
type fakeSender struct {
	mu   sync.Mutex
	sent [][]byte
	err  error
}

func (f *fakeSender) Send(payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}
	// Copy, because a broadcast hands the same slice to every client and a
	// real connection hands those bytes to the kernel.
	f.sent = append(f.sent, append([]byte(nil), payload...))
	return nil
}

// received decodes everything the sender was given.
func (f *fakeSender) received(t *testing.T) []Message {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]Message, 0, len(f.sent))
	for _, payload := range f.sent {
		var msg Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("decode %q: %v", payload, err)
		}
		out = append(out, msg)
	}
	return out
}

// joinAll joins every client, failing the test if a join is refused.
func joinAll(t *testing.T, room *Room, clients ...*Client) {
	t.Helper()

	for _, c := range clients {
		if err := room.Join(c); err != nil {
			t.Fatalf("join %s: %v", c.Name(), err)
		}
	}
}

func TestBroadcastReachesEveryMember(t *testing.T) {
	room := NewRoom("lobby")

	aliceSender, bobSender := &fakeSender{}, &fakeSender{}
	alice := NewClient("alice", aliceSender)
	joinAll(t, room, alice, NewClient("bob", bobSender))

	if err := room.Publish(alice, "  hello  "); err != nil {
		t.Fatalf("publish: %v", err)
	}

	for name, sender := range map[string]*fakeSender{"alice": aliceSender, "bob": bobSender} {
		got := sender.received(t)
		if len(got) != 1 {
			t.Fatalf("%s received %d messages, want 1", name, len(got))
		}
		if got[0].Type != MsgChat || got[0].From != "alice" || got[0].Text != "hello" {
			t.Fatalf("%s received %+v", name, got[0])
		}
		if got[0].Room != "lobby" {
			t.Fatalf("%s: room = %q, want lobby", name, got[0].Room)
		}
		if got[0].Time.IsZero() {
			t.Fatalf("%s: the room did not stamp a time", name)
		}
	}
}

func TestJoinRejectsATakenName(t *testing.T) {
	room := NewRoom("lobby")
	joinAll(t, room, NewClient("alice", &fakeSender{}))

	err := room.Join(NewClient("alice", &fakeSender{}))
	if !errors.Is(err, ErrNameTaken) {
		t.Fatalf("err = %v, want ErrNameTaken", err)
	}
	if room.Len() != 1 {
		t.Fatalf("len = %d, want 1", room.Len())
	}
}

func TestLeaveRemovesTheClient(t *testing.T) {
	room := NewRoom("lobby")

	sender := &fakeSender{}
	joinAll(t, room, NewClient("alice", sender))

	room.Leave("alice")
	room.Leave("alice") // leaving twice must stay a no-op

	room.Broadcast(Message{Type: MsgSystem, Text: "everyone"})

	if got := len(sender.received(t)); got != 0 {
		t.Fatalf("alice received %d messages after leaving, want 0", got)
	}
	if room.Len() != 0 {
		t.Fatalf("len = %d, want 0", room.Len())
	}
}

func TestPublishRejectsBlankAndOversizedMessages(t *testing.T) {
	room := NewRoom("lobby")
	alice := NewClient("alice", &fakeSender{})
	joinAll(t, room, alice)

	if err := room.Publish(alice, "   "); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("blank message err = %v, want ErrEmptyMessage", err)
	}
	long := strings.Repeat("a", MaxMessageLen+1)
	if err := room.Publish(alice, long); !errors.Is(err, ErrMessageTooLong) {
		t.Fatalf("long message err = %v, want ErrMessageTooLong", err)
	}
}

func TestOneFailingSenderDoesNotStopTheBroadcast(t *testing.T) {
	room := NewRoom("lobby")

	broken := &fakeSender{err: errors.New("queue full")}
	bobSender := &fakeSender{}
	joinAll(t, room,
		NewClient("alice", broken),
		NewClient("bob", bobSender),
	)

	room.Broadcast(Message{Type: MsgSystem, Text: "hello"})

	got := bobSender.received(t)
	if len(got) != 1 || got[0].Text != "hello" {
		t.Fatalf("bob received %+v, want one system message", got)
	}
}

func TestSendReachesOnlyTheNamedClient(t *testing.T) {
	room := NewRoom("lobby")

	aliceSender, bobSender := &fakeSender{}, &fakeSender{}
	alice := NewClient("alice", aliceSender)
	joinAll(t, room, alice, NewClient("bob", bobSender))

	if err := room.Send(alice, Message{Type: MsgPresence, Members: room.Members()}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if got := len(bobSender.received(t)); got != 0 {
		t.Fatalf("bob received %d messages, want 0", got)
	}
	got := aliceSender.received(t)
	if len(got) != 1 || got[0].Type != MsgPresence || len(got[0].Members) != 2 {
		t.Fatalf("alice received %+v", got)
	}
}

func TestMembersAreSorted(t *testing.T) {
	room := NewRoom("lobby")
	for _, name := range []string{"carol", "alice", "bob"} {
		joinAll(t, room, NewClient(name, &fakeSender{}))
	}

	if got := strings.Join(room.Members(), ","); got != "alice,bob,carol" {
		t.Fatalf("members = %q", got)
	}
}

// TestRoomIsSafeForConcurrentUse has no assertion of its own: with -race, the
// race detector is the assertion.
func TestRoomIsSafeForConcurrentUse(t *testing.T) {
	room := NewRoom("lobby")

	const clients = 8
	for i := range clients {
		name := fmt.Sprintf("client-%d", i)
		joinAll(t, room, NewClient(name, &fakeSender{}))
	}

	var wg sync.WaitGroup

	for i := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("client-%d", i)
			for range 50 {
				room.Broadcast(Message{Type: MsgChat, From: name, Text: "hi"})
				_ = room.Members()
				_ = room.Len()
			}
		}()
	}

	for i := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("client-%d", i)
			room.Leave(name)
			_ = room.Join(NewClient(name, &fakeSender{}))
		}()
	}

	wg.Wait()
}
