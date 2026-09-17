package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

var errQueueFull = errors.New("Outbound queue is full")

// connection is the outboundhalf of one WebSocket: a buffered queue, the write pump that drains it,
// and the policy that a client which cannot keep up is dropped instead of stallin the room.
//
// It implements chat.Sender, which is everything the domain knows about it.
type connection struct {
	ws  *websocket.Conn
	out chan []byte
	log *slog.Logger

	mu     sync.Mutex
	closed bool
}

// newConnection returns a connection ready to hand to chat.Newclient.
//
// The queue must be at least one deep: the first messages (a presence snapshot and the join announcement) are queued
// before the write pump starts.
func newConnection(ws *websocket.Conn, queue int, log *slog.Logger) *connection {
	return &connection{
		ws:  ws,
		out: make(chan []byte, queue),
		log: log,
	}
}

// Send queues payload for this client.
//
// Send never blocks: when the queue is full the client is dropped, because one slow reader
// must not hold up the room that is broadcasting to everybody else.
func (c *connection) Send(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	select {
	case c.out <- payload:
		return nil
	default:
		c.drop("Connction too slow to keep up with messages")
		return errQueueFull
	}
}

// writePump is the only goroutine that writes to the socket. Keeping the write
// side in one place preserves the order messages were queued in, and it maches the library's rule
// that reads are exclusive while writes are not.
func (c *connection) writePump(ctx context.Context) error {
	for {
		select {
		case payload := <-c.out:
			if err := c.write(ctx, payload); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// write sends one message with a deadline, so a stalled peer cannot pin the
// write pump, and with it this client's queue, forever.
func (c *connection) write(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, writeWait)
	defer cancel()

	return c.ws.Write(ctx, websocket.MessageText, payload)
}

// pingLoop detects clients that vanished without a close handshake.
// Ping needs a concurrent reader, which is why the read loop exists.
func (c *connection) pingLoop(ctx context.Context) error {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := c.ws.Ping(ctx)
			cancel()
			if err != nil {
				return fmt.Errorf("ping: %w", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// drop closes the connection with a policy violation, once. That close code is
// how a well behaved client learns not to bother reconnecting.
func (c *connection) drop(reason string) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()

	c.log.Warn("Closing connection", "reason", reason)

	// Close performs a close handshake, which can block for seconds, so it must not run
	// on the caller's goroutine: the caller may be a room holding a lock while it broadcasts.
	go func() {
		if err := c.ws.Close(websocket.StatusPolicyViolation, reason); err != nil {
			c.log.Debug("Close connection", "error", err)
		}
	}()
}

// isClosed reports whether the connection has been dropped.
func (c *connection) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
