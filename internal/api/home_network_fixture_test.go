package api

import "ams/internal/auth"

// HTTP fixtures use this synthetic LAN, independently of the test runner's network interfaces.
func init() { nets, _, _ := auth.ParseNetworks("192.168.1.0/24"); auth.SetHomeNetworks(nets) }
