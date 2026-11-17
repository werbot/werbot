package app

import (
	_ "github.com/werbot/werbot/internal/connector/ssh"    // registers the ssh resource type
	_ "github.com/werbot/werbot/internal/connector/tcp"    // registers the tcp resource type
	_ "github.com/werbot/werbot/internal/connector/telnet" // registers the telnet resource type
	_ "github.com/werbot/werbot/internal/connector/udp"    // registers the udp resource type
)
