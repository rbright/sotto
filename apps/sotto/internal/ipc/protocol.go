// Package ipc provides single-instance unix-socket protocol and server/client helpers.
package ipc

// Request is one command sent over the local unix-domain socket.
type Request struct {
	Command string `json:"command"`
}

// Response is the normalized command outcome returned by the owner session.
type Response struct {
	OK      bool   `json:"ok"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
