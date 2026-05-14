// Package relay forwards Cursor-format requests to LLM provider APIs,
// rewriting request and response bodies to match the expected formats.
package relay

import (
	"strings"
)

const connectFlagCompressed = 0x01
const connectFlagEndStream = 0x02

func isCursorRPC(path string) bool {
	return strings.HasPrefix(path, "/aiserver.v1.") ||
		strings.HasPrefix(path, "/agent.v1.") ||
		strings.HasPrefix(path, "/anyrun.v1.") ||
		strings.HasPrefix(path, "/internapi.v1.")
}

type Gateway struct {
	adapterProvider func() []AdapterInfo
}

// SetAdapterProvider installs a callback that returns the user's currently
// configured BYOK adapters. Called by ProxyService.
func (g *Gateway) SetAdapterProvider(fn func() []AdapterInfo) { g.adapterProvider = fn }

func NewGateway() *Gateway {
	return &Gateway{}
}