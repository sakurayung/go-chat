// Package transport is the I/O boundary of the chat server: it owns HTTP requests and WebSocket connections, and depends on internal/chat for the rules and the state.
//
// Nothing here decides what a room is or who may join it. The transport turns a request into domain calls,
// and domain messages into frames.
package transport

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/sakurayung/gochat/internal/chat"
	"golang.org/x/time/rate"
)

const (
	// defaultOutboundQueue is how many messages may be waiting for one client before that client is dropped.
	defaultOutboundQueue = 16

	// maxClientMessageLen caps a single frame a client may send us, so a peer cannot decide how much memory we allocate.
	maxClientMessageLen = 4096

	// writeWait is how long writing one message to one client may take before that client is considered dead.
	writeWait = 5 * time.Second

	// pingInterval and pingTimeout detect clients that disappeared without a close handshake (closed lid, unplugged cable, ...).
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
)

// Config configures a Server. Every field has a working default except Rooms.
type Config struct {
	// Rooms is the chat registry this server exposes. When nil an empty registry is created, which is what most tests want.
	Rooms           *chat.Registry
	Logger          *slog.Logger
	Assets          http.Handler
	AnnounceLimiter *rate.Limiter
	OutboundQueue   int
}

// Server serves the chat over HTTP: a WebSocket endpoint for clients, an announce endpoint for
// the operator, a room listing and the static front end.
type Server struct {
	cfg Config
	log *slog.Logger
	mux *http.ServeMux
	// conns tracks every live WebSocket. http.Server does not manage hijacked connections,
	// so shutdown has to find them here.
	connsMu sync.Mutex
	conns   map[*websocket.Conn]struct{}
}

func NewServer(cfg Config) *Server {
	if cfg.Rooms == nil {
		cfg.Rooms = chat.NewRegistry()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Assets == nil {
		cfg.Assets = http.NotFoundHandler()
	}
	if cfg.AnnounceLimiter == nil {
		cfg.AnnounceLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 8)
	}
	if cfg.OutboundQueue < 1 {
		cfg.OutboundQueue = defaultOutboundQueue
	}
	s := &Server{
		cfg:   cfg,
		log:   cfg.Logger,
		mux:   http.NewServeMux(),
		conns: make(map[*websocket.Conn]struct{}),
	}
	s.mux.HandleFunc("GET /subscribe", s.subscribeHandler)
	s.mux.HandleFunc("POST /announce", s.announceHandler)
	s.mux.HandleFunc("GET /rooms", s.roomsHandler)
	s.mux.Handle("/", cfg.Assets)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) trackConn(ws *websocket.Conn) {
	s.connsMu.Lock()
	s.conns[ws] = struct{}{}
	s.connsMu.Unlock()
}

func (s *Server) untrackConn(ws *websocket.Conn) {
	s.connsMu.Lock()
	delete(s.conns, ws)
	s.connsMu.Unlock()
}

func (s *Server) DisconnectAll() {
	s.connsMu.Lock()
	conns := make([]*websocket.Conn, 0, len(s.conns))
	for ws := range s.conns {
		conns = append(conns, ws)
	}
	s.connsMu.Unlock()

	var wg sync.WaitGroup
	for _, ws := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ws.Close(websocket.StatusGoingAway, "server shutting down"); err != nil {
				s.log.Debug("Close connection while shutting down", "error", err)
			}
		}()
	}
	wg.Wait()
}
