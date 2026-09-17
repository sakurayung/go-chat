package transport

import (
	"io"
	"net/http"
	"strings"

	"github.com/sakurayung/gochat/internal/chat"
)

func (s *Server) announceHandler(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, chat.MaxMessageLen)
	text, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}

	if !s.cfg.AnnounceLimiter.Allow() {
		w.Header().Set("Retry-After", "1")
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	msg := strings.TrimSpace(string(text))
	if msg == "" {
		http.Error(w, "empty announcement", http.StatusBadRequest)
		return
	}
	s.announce(msg)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) announce(text string) {
	for _, room := range s.cfg.Rooms.All() {
		room.Broadcast(chat.ServerMessage(chat.MsgSystem, text))
	}
}
