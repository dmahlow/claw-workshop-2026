package a2a

// AgentCard describes an agent's identity and capabilities per the A2A spec.
type AgentCard struct {
	Name               string       `json:"name"`
	Description        string       `json:"description"`
	URL                string       `json:"url"`
	Version            string       `json:"version"`
	Skills             []AgentSkill `json:"skills"`
	DefaultInputModes  []string     `json:"defaultInputModes"`
	DefaultOutputModes []string     `json:"defaultOutputModes"`
}

// AgentSkill describes a single capability of an agent.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// JSONRPCRequest is a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      interface{} `json:"id"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is the error object in a JSON-RPC 2.0 response.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SendMessageParams contains the parameters for a message/send request.
type SendMessageParams struct {
	Message Message `json:"message"`
}

// Message represents an A2A protocol message with a role and parts.
type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// Part is a content part within a message (text only for now).
type Part struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// SendMessageResult is the result of a message/send call.
type SendMessageResult struct {
	Message Message `json:"message"`
}
