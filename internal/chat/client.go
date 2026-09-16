package chat

// Sender delivers an already encoded message to one client.
type Sender interface {
	Send(payload []byte) error
}

// Client is one person, as seen by a room.
type Client struct {
	name   string
	sender Sender
}

// NewClient returns a client that is reachable through sender.
func NewClient(name string, sender Sender) *Client {
	return &Client{
		name:   name,
		sender: sender,
	}
}

// Name returns the name the client joined with.
func (c *Client) Name() string {
	return c.name
}
