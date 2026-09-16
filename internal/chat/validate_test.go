package chat

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"alice", false},
		{"a-b_c9", false},
		{"a", true}, // shorter than MinNameLen
		{strings.Repeat("a", MaxNameLen+1), true},
		{"", true},
		{"admin", true},     // reserved, any case
		{"SERVER", true},    // reserved
		{"alice bob", true}, // spaces are not in the allow-list
		{"héllo", true},
	}

	for _, tt := range tests {
		err := ValidateName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateName(%q) = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if err != nil && !errors.Is(err, ErrInvalidName) {
			t.Errorf("ValidateName(%q) = %v, want it to wrap ErrInvalidName", tt.name, err)
		}
	}
}

func TestValidateRoomName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"lobby", false},
		{"side-project-2", false},
		{"", true},
		{strings.Repeat("a", MaxRoomNameLen+1), true},
		{"Lobby", true}, // upper case is not in the allow-list
		{"side project", true},
	}

	for _, tt := range tests {
		err := ValidateRoomName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateRoomName(%q) = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if err != nil && !errors.Is(err, ErrInvalidRoom) {
			t.Errorf("ValidateRoomName(%q) = %v, want it to wrap ErrInvalidRoom", tt.name, err)
		}
	}
}

// TestLimitsAreExported documents why the numbers live here: the transport sizes
// its read limit and query parsing from the same values.
func TestLimitsAreExported(t *testing.T) {
	if MinNameLen >= MaxNameLen {
		t.Fatal("MinNameLen must be smaller than MaxNameLen")
	}
	if DefaultRoom == "" || ServerName == "" || MaxMessageLen <= 0 || MaxRoomNameLen <= 0 {
		t.Fatal("limits must be set")
	}
}
