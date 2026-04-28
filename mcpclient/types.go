package mcpclient

import (
	mcpTypes "github.com/yaoapp/gou/mcp/types"
)

// Client wraps mcpTypes.ClientDSL with Registry management fields.
// Uses ClientDSL.ID as the registry key.
type Client struct {
	mcpTypes.ClientDSL

	RuntimeID string          `json:"runtime_id"`
	Enabled   bool            `json:"enabled"`
	Status    string          `json:"status"`
	Source    ClientSource    `json:"source"`
	ToolList  []mcpTypes.Tool `json:"tool_list,omitempty"`
	Owner     ClientOwner     `json:"owner"`
}

// ClientOwner identifies who owns a client entry.
type ClientOwner struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

// ClientSource distinguishes registry-created from DSL-loaded clients.
type ClientSource string

const (
	ClientSourceDynamic ClientSource = "dynamic"
	ClientSourceBuiltIn ClientSource = "builtin"
	ClientSourceAll     ClientSource = "all"
)

// ClientFilter specifies criteria for listing clients.
type ClientFilter struct {
	Owner     *ClientOwner
	Enabled   *bool
	Source    ClientSource
	Transport *mcpTypes.TransportType
	Type      *string
	Keyword   string
}

// ClientTestResult holds the outcome of a client connectivity test.
type ClientTestResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}
