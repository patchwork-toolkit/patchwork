package main

import (
	"time"
)

const (
	// Use to invalidate cache during the requests for agent's data
	AgentResponseCacheTTL time.Duration = time.Duration(3) * time.Second

	// DNS-SD service name (type)
	DnssdServiceType = "_pw-dgw._tcp"
)
