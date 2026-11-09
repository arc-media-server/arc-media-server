//go:build linux

package auth

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func interfaceKinds() map[string]string {
	result := map[string]string{}
	data, err := syscall.NetlinkRIB(syscall.RTM_GETLINK, syscall.AF_UNSPEC)
	if err == nil {
		messages, _ := syscall.ParseNetlinkMessage(data)
		for _, message := range messages {
			if message.Header.Type != syscall.RTM_NEWLINK {
				continue
			}
			attrs, err := syscall.ParseNetlinkRouteAttr(&message)
			if err != nil {
				continue
			}
			name, kind := "", ""
			for _, attr := range attrs {
				switch attr.Attr.Type & 0x3fff {
				case syscall.IFLA_IFNAME:
					name = strings.TrimRight(string(attr.Value), "\x00")
				case 18: // IFLA_LINKINFO, with nested IFLA_INFO_KIND=1.
					for rest := attr.Value; len(rest) >= 4; {
						size := int(binary.NativeEndian.Uint16(rest[:2]))
						if size < 4 || size > len(rest) {
							break
						}
						if binary.NativeEndian.Uint16(rest[2:4])&0x3fff == 1 {
							kind = strings.TrimRight(string(rest[4:size]), "\x00")
						}
						step := (size + 3) &^ 3
						if step > len(rest) {
							break
						}
						rest = rest[step:]
					}
				}
			}
			if name != "" {
				result[name] = kind
			}
		}
	}
	// Fail closed if native link metadata is unavailable: do not guess that a hidden
	// virtual interface is a LAN merely because its address is private.
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if _, known := result[iface.Name]; !known {
			result[iface.Name] = "unknown-virtual"
		}
	}
	return result
}
func defaultGateways() []net.IP {
	data, _ := os.ReadFile("/proc/net/route")
	v6, _ := os.ReadFile("/proc/net/ipv6_route")
	return parseGateways(data, v6)
}
func parseGateways(data, v6 []byte) []net.IP {
	var ips []net.IP
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != "00000000" {
			continue
		}
		value, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil || value == 0 {
			continue
		}
		ips = append(ips, net.IPv4(byte(value), byte(value>>8), byte(value>>16), byte(value>>24)))
	}
	for _, line := range strings.Split(string(v6), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[0] != strings.Repeat("0", 32) || fields[1] != "00" {
			continue
		}
		value, err := hex.DecodeString(fields[4])
		if err != nil || len(value) != 16 {
			continue
		}
		ip := net.IP(value)
		if !ip.IsUnspecified() {
			ips = append(ips, ip)
		}
	}
	return ips
}
