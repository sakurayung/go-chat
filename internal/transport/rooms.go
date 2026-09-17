package transport

import (
	"encoding/json"
	"net/http"
	"sort"
)

type rooomJSON struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

func (s *Server) roomsHandler(w http.ResponseWriter, r *http.Request) {
	rooms := s.cfg.Rooms.All()

	views := make([]rooomJSON, 0, len(rooms))
	for _, room := range rooms {
		views = append(views, rooomJSON{
			Name:    room.Name(),
			Members: room.Members(),
		})
	}
	sort.Slice(views, func(i, j int) bool {
		return views[i].Name < views[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(views); err != nil {
		s.log.Warn("encode rooms", "error", err)
	}
}
