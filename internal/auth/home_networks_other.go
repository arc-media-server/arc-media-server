//go:build !linux && !windows

package auth

import "net"

func interfaceKinds() map[string]string { return nil }
func defaultGateways() []net.IP         { return nil }
