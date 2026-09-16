package chat

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultRoom = "lobby"
	ServerName  = "server"

	MinNameLen = 2
	MaxNameLen = 20

	MaxRoomNameLen = 24
	MaxMessageLen  = 512
)

var reservedNames = map[string]struct{}{
	ServerName: {},
	"system":   {},
	"admin":    {},
}

func ValidateName(name string) error {
	if n := utf8.RuneCountInString(name); n < MinNameLen || n > MaxNameLen {
		return fmt.Errorf("%w: must be between %d and %d characters", ErrInvalidName, MinNameLen, MaxNameLen)
	}
	if _, ok := reservedNames[strings.ToLower(name)]; ok {
		return fmt.Errorf("%w: %q is reserved", ErrInvalidName, name)
	}

	for _, r := range name {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9', r == '_', r == '-':
		default:
			return fmt.Errorf("%w: only letters, digits, '-' and '_' are allowed", ErrInvalidName)
		}
	}
	return nil
}

func ValidateRoomName(name string) error {
	if name == "" || len(name) > MaxRoomNameLen {
		return fmt.Errorf("%w: must be 1 to %d characters", ErrInvalidRoom, MaxRoomNameLen)
	}
	for _, r := range name {
		switch {
		case 'a' <= r && r <= 'z', '0' <= r && r <= '9', r == '-':
		default:
			return fmt.Errorf("%w: only lower case letters, digits and '-' are allowed", ErrInvalidRoom)
		}
	}
	return nil
}
