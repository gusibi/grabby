package browser

// BrowserRegistration stores a registered browser mapping.
type BrowserRegistration struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

// BrowserInfo represents a connected browser instance.
type BrowserInfo struct {
	ConnID string `json:"conn_id"`
	Name   string `json:"name"`
}
