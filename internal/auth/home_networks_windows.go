//go:build windows

package auth

import (
	"golang.org/x/sys/windows"
	"net"
	"unsafe"
)

func interfaceKinds() map[string]string { return nil }
func defaultGateways() []net.IP {
	size := uint32(15000)
	for range 3 {
		if size > 1<<20 {
			return nil
		}
		data := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&data[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_GATEWAYS, 0, first, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil
		}
		var ips []net.IP
		for adapter := first; adapter != nil; adapter = adapter.Next {
			for gateway := adapter.FirstGatewayAddress; gateway != nil; gateway = gateway.Next {
				if ip := gateway.Address.IP(); ip != nil && !ip.IsUnspecified() {
					ips = append(ips, ip)
				}
			}
		}
		return ips
	}
	return nil
}
