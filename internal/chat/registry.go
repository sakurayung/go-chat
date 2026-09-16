package chat

import "sync"

// Registry keeps the rooms of a server
//
// A room with no members left is forgotten, so a long running server does not
// accumulate empty rooms. Nothing here knows about HTTP....
type Registry struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewRegistry() *Registry {
	return &Registry{
		rooms: make(map[string]*Room),
	}
}

func (g *Registry) GetOrCreate(name string) *Room {
	g.mu.RLock()
	room, ok := g.rooms[name]
	g.mu.RUnlock()
	if ok {
		return room
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	// Check again: another goroutine may have created the room betwen the two critical sections.
	// Both checks are needed, the first one keps the common case read only.
	if room, ok := g.rooms[name]; ok {
		return room
	}
	room = NewRoom(name)
	g.rooms[name] = room
	return room
}

func (g *Registry) DropIfEmpty(room *Room) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Re-check under the write lock: somebody may have joined between the caller's Leave and this line, and forgetting the room would strand them
	// in a room nobody can reach any more.
	if room.Len() == 0 {
		delete(g.rooms, room.Name())
	}
}

// All returns every room as a snapshot, so the caller can work with the slice
// without holding the registry lock.
func (g *Registry) All() []*Room {
	g.mu.RLock()
	defer g.mu.RUnlock()

	rooms := make([]*Room, 0, len(g.rooms))
	for _, room := range g.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}
