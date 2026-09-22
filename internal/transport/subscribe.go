package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/sakurayung/gochat/internal/chat"
)

func (s *Server) subscribeHandler(w http.ResponseWriter, r *http.Request) {
	err := s.subscribe(w, r)
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	// A client that closed the tab is normal traffic, not an error.
	if code := websocket.CloseStatus(err); code == websocket.StatusNormalClosure || code == websocket.StatusGoingAway {
		return
	}
	s.log.Warn("subscribe", "error", err)
}

// subscribe joins the caller to a room and serves the connection until it ends.
func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()

	name := strings.TrimSpace(query.Get("name"))
	if err := chat.ValidateName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	roomName := strings.TrimSpace(query.Get("room"))
	if roomName == "" {
		roomName = chat.DefaultRoom
	}
	if err := chat.ValidateRoomName(roomName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return err
	}

	defer ws.CloseNow()

	s.trackConn(ws)
	defer s.untrackConn(ws)

	ws.SetReadLimit(maxClientMessageLen)

	room := s.cfg.Rooms.GetOrCreate(roomName)
	conn := newConnection(ws, s.cfg.OutboundQueue, s.log)
	client := chat.NewClient(name, conn)

	if err := room.Join(client); err != nil {
		s.rejectJoin(conn, err)
		return fmt.Errorf("join: %s: %w", roomName, err)
	}
	defer s.leave(room, client)

	if err := room.Send(client, chat.Message{
		Type:    chat.MsgPresence,
		Members: room.Members(),
	}); err != nil {
		return err
	}
	room.Broadcast(chat.Message{
		Type: chat.MsgJoin,
		From: name,
	})
	return s.serve(conn, room, client)
}

// serve runs the three goroutines that own a connection and returns as soon as one of them fails
func (s *Server) serve(conn *connection, room *chat.Room, client *chat.Client) error {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	errc := make(chan error, 1)

	// fail keeps only the first error: that is the reason the connection ended.
	fail := func(err error) {
		select {
		case errc <- err:
		default:
		}
	}
	// There's a new extended library to implement this much cleaner
	wg.Add(3)
	go func() { defer wg.Done(); fail(conn.writePump(ctx)) }()
	go func() { defer wg.Done(); fail(conn.pingLoop(ctx)) }()
	go func() { defer wg.Done(); fail(s.readLoop(ctx, room, client, conn)) }()

	err := <-errc
	cancel()
	wg.Wait()
	return err
}

// readLoop is the only goroutine that reads from the connection. It never trusts what it reads:
// only Type and Text survive, everything else is the server's.
func (s *Server) readLoop(ctx context.Context, room *chat.Room, client *chat.Client, conn *connection) error {
	for {
		var msg chat.Message
		if err := wsjson.Read(ctx, conn.ws, &msg); err != nil {
			return err
		}

		switch msg.Type {
		case chat.MsgChat:
			if err := room.Publish(client, msg.Text); err != nil {
				// The client asked for something we will not do; say so and keep the connection.
				_ = room.Send(client, chat.ServerMessage(chat.MsgError, err.Error()))
			}
		default:
			_ = room.Send(client, chat.ServerMessage(chat.MsgError, fmt.Sprintf("Unsupported message type %q", msg.Type)))
		}
	}
}

func (s *Server) leave(room *chat.Room, client *chat.Client) {
	room.Leave(client.Name())
	room.Broadcast(chat.Message{
		Type: chat.MsgLeave,
		From: client.Name(),
	})
	s.cfg.Rooms.DropIfEmpty(room)
}

// rejectJoin tells a client why it was refused and closes the connection with a policy violation.
//
// The message is written straight to the socket because the write pump has not started yet: a connection
// that never joined never needs one
func (s *Server) rejectJoin(conn *connection, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), writeWait)
	defer cancel()

	payload, encErr := chat.Encode(chat.ServerMessage(chat.MsgError, err.Error()))
	if encErr == nil {
		_ = conn.write(ctx, payload)
	}

	/*
	 * 	rejectJoin closes the socket without sending a WebSocket close frame by this code below
	 */
	// conn.drop("join rejected")
	_ = conn.ws.Close(websocket.StatusPolicyViolation, "name already taken")
}
