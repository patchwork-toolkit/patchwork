package main

import (
	"time"
)

const (
	// Use to invalidate cache during the requests for agent's data
	AgentResponseCacheTTL time.Duration = time.Duration(3) * time.Second

	// DNS-SD service name (type)
	DnssdServiceType = "_pw-dgw._tcp"

	// Static resources URL mounting point
	StaticLocation = "/static"

	// Device Catalog URL mounting point
	CatalogLocation = "/dc"
)
