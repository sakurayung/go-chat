package transport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket/wsjson"
	"github.com/sakurayung/gochat/internal/chat"
)

func TestChatReachesEveryoneInTheRoom(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	alice := dialJoined(t, ts, "alice", "lobby")
	bob := dial(t, ts, "bob", "lobby")

	sendChat(t, bob, "hello alice")

	// Bob gets his own message back: the server is the source of truth for ordering, timestamps and the sender name.
	echo := waitFor(t, bob, chat.MsgChat)
	if echo.From != "bob" || echo.Text != "hello alice" {
		t.Fatalf("echo = %+v", echo)
	}
	if echo.Room != "lobby" {
		t.Fatalf("echo.Room = %q, want lobby", echo.Room)
	}
	if echo.Time.IsZero() {
		t.Fatal("the server did not stamp a time")
	}

	got := waitFor(t, alice, chat.MsgChat)
	if got.From != "bob" || got.Text != "hello alice" {
		t.Fatalf("alice saw %+v", got)
	}
}

func TestPresenceSnapshotsMembers(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	dialJoined(t, ts, "alice", "lobby")

	// bob reads his own presence snapshot, so it must list both of them sorted:
	// two clients rendering the same room have to agree
	bob := dial(t, ts, "bob", "lobby")
	presence := waitFor(t, bob, chat.MsgPresence)

	if strings.Join(presence.Members, ",") != "alice,bob" {
		t.Fatalf("members = %v, want [alice bob]", presence.Members)
	}
}

func TestRoomsAreIsolated(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})
	alice := dialJoined(t, ts, "alice", "lobby")
	carol := dialJoined(t, ts, "carol", "games")

	// carol must not see anything that happens in lobby.
	sendChat(t, alice, "lobby only")

	echo := waitFor(t, alice, chat.MsgChat)
	if echo.Text != "lobby only" {
		t.Fatalf("echo = %+v", echo)
	}

	// carol's next message is her own join announcement: nothing from lobby was
	// queued for her.
	next := readMessage(t, carol)
	if next.Type != chat.MsgJoin || next.From != "carol" {
		t.Fatalf("carol received %+v, want her own join", next)
	}
}

func TestClientCannotSpoofIdentity(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	alice := dialJoined(t, ts, "alice", "lobby")
	bob := dialJoined(t, ts, "bob", "lobby")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// A hostile client sets every field the server owns.
	err := wsjson.Write(ctx, bob, chat.Message{
		Type: chat.MsgChat,
		Room: "games",
		From: "alice",
		Text: "i am alice",
		Time: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	got := waitFor(t, alice, chat.MsgChat)
	if got.From != "bob" {
		t.Fatalf("From = %q, want bob", got.From)
	}
	if got.Room != "lobby" {
		t.Fatalf("Room = %q, want lobby", got.Room)
	}
	if got.Time.Year() == 1999 {
		t.Fatal("the client's timestamp survived, it must be overwritten")
	}
}

func TestDefaultRoomIsUsedWhenNoneIsAsked(t *testing.T) {
	ts, _, _ := newTestServer(t, Config{})

	dialJoined(t, ts, "alice", "")

	views := roomList(t, ts)
	if len(views) != 1 || views[0].Name != chat.DefaultRoom {
		t.Fatalf("rooms = %+v, want just %q", views, chat.DefaultRoom)
	}
}
