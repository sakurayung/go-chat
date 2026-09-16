package chat

import "testing"

func TestGetOrCreateReusesRooms(t *testing.T) {
	reg := NewRegistry()

	if reg.GetOrCreate("lobby") != reg.GetOrCreate("lobby") {
		t.Fatal("GetOrCreate returned two rooms for one name")
	}
	if reg.GetOrCreate("lobby") == reg.GetOrCreate("games") {
		t.Fatal("GetOrCreate returned one room for two names")
	}
	if got := len(reg.All()); got != 2 {
		t.Fatalf("rooms = %d, want 2", got)
	}
}

func TestDropIfEmptyForgetsOnlyEmptyRooms(t *testing.T) {
	reg := NewRegistry()
	room := reg.GetOrCreate("lobby")
	joinAll(t, room, NewClient("alice", &fakeSender{}))

	// A room with members stays.
	reg.DropIfEmpty(room)
	if got := len(reg.All()); got != 1 {
		t.Fatalf("rooms = %d, want 1", got)
	}

	// The last member leaving forgets the room.
	room.Leave("alice")
	reg.DropIfEmpty(room)
	if got := len(reg.All()); got != 0 {
		t.Fatalf("rooms = %d, want 0", got)
	}
}

func TestAllReturnsASnapshot(t *testing.T) {
	reg := NewRegistry()
	reg.GetOrCreate("lobby")

	snapshot := reg.All()
	reg.GetOrCreate("games")

	// The snapshot was taken before games was created, so it must not grow:
	// callers can hold it while they work.
	if len(snapshot) != 1 {
		t.Fatalf("snapshot = %d rooms, want 1", len(snapshot))
	}
	if len(reg.All()) != 2 {
		t.Fatalf("registry = %d rooms, want 2", len(reg.All()))
	}
}
