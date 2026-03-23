package domain

import (
	coreclient "github.com/gotunnel/internal/core/client"
	corerule "github.com/gotunnel/internal/core/rule"
)

// Client is retained as a cohesive aggregate alias for server-side callers.
type Client = coreclient.Client

// ProxyRule is retained as a cohesive aggregate alias for server-side callers.
type ProxyRule = corerule.ProxyRule
