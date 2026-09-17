package chat

import (
	"encoding/json"
	"time"
)

type MessageType string

const (
	MsgChat     MessageType = "chat"
	MsgJoin     MessageType = "join"
	MsgLeave    MessageType = "leave"
	MsgSystem   MessageType = "system"
	MsgPresence MessageType = "presence"
	MsgError    MessageType = "error"
)

type Message struct {
	Type    MessageType `json:"type"`
	Room    string      `json:"room,omitempty"`
	From    string      `json:"from,omitempty"`
	Text    string      `json:"text,omitempty"`
	Time    time.Time   `json:"time"`
	Members []string    `json:"members,omitempty"`
}

// Encode turns msg into the bytes that go on the wire
// The envelope is the domain's vocabulary, so the domain own its encoding and the transport only moves the bytes.
func Encode(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

// ServerMessage returns a message the server writes itself: From and Time are already filled in.
//
// A caller that goes through a room gets both done by Room.encode.
// This is for the paths that cannot, such as refusing a join before the client is in a room, so the rule
// "only the server stamps a time" stays in one package.
func ServerMessage(msgType MessageType, text string) Message {
	return Message{
		Type: msgType,
		From: ServerName,
		Text: text,
		Time: time.Now().UTC(),
	}
}
