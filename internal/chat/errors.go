package chat

import "errors"

var (
	ErrNameTaken      = errors.New("Name is already taken in this room")
	ErrInvalidName    = errors.New("Invalid name")
	ErrInvalidRoom    = errors.New("Invalid room name")
	ErrEmptyMessage   = errors.New("Message is empty")
	ErrMessageTooLong = errors.New("Message is too long")
)
