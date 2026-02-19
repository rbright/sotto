package ipc

type Request struct {
	Command string `json:"command"`
}

type Response struct {
	OK      bool   `json:"ok"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
