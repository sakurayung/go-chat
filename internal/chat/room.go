package chat

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// A Room is safe for concurrent use by many goroutines: every read and write of its state happen under its mutex.
type Room struct {
	name string

	// A RWMutex, because broadcasting and listing members happen far more often than joining and leaving.
	//
	// Clients holds pointers, never values: two clients with the same fields are still two different people, so identity has to be the pointer.
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewRoom(name string) *Room {
	return &Room{
		name:    name,
		clients: make(map[string]*Client),
	}
}

func (r *Room) Name() string {
	return r.name
}

func (r *Room) Join(client *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.clients[client.name]; ok {
		return fmt.Errorf("%w: %s", ErrNameTaken, client.name)
	}
	r.clients[client.name] = client
	return nil
}

func (r *Room) Leave(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, name)
}

func (r *Room) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Room) Members() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Publish validates the text a client sent and broadcasts it to the room.
func (r *Room) Publish(from *Client, text string) error {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return ErrEmptyMessage
	case len(text) > MaxMessageLen:
		return fmt.Errorf("%w: at most %d bytes", ErrMessageTooLong, MaxMessageLen)
	}
	r.Broadcast(Message{
		Type: MsgChat,
		From: from.name,
		Text: text,
	})
	return nil
}

// Broadcast stamps msg and queues it for every client in the room.
//
// Broadcast never blocks. A client that cannot keep up is skipped, because one slow reader must not hold up the room; the transport notices the error from
// the sender and closes that connection, which removes it through the normal levae path.
func (r *Room) Broadcast(msg Message) {
	payload, err := r.encode(msg)
	if err != nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.clients {
		_ = c.sender.Send(payload)
	}
}

// Send queues one message for one client.
func (r *Room) Send(client *Client, msg Message) error {
	payload, err := r.encode(msg)
	if err != nil {
		return err
	}
	return client.sender.Send(payload)
}

// encode stamps msg and turns it into wire bytes.
//
// One encoding is shared by every recipient of a broadcast, so the cost of a broadcast is one allocation no matter how many clients are in the room.
func (r *Room) encode(msg Message) ([]byte, error) {
	msg.Room = r.name
	msg.Time = time.Now().UTC()
	return Encode(msg)
}
