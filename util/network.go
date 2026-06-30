package util

import (
	"fmt"
	"net"

	"epos-proxy/logger"
)

func GetLocalIP() (string, error) {
	logger.Infof("Detecting local LAN IP address...")

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()

		logger.Debugf("UDP dial successful, checking local address...")

		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if ip := addr.IP.To4(); ip != nil {
				logger.Infof("Detected LAN IP via UDP route: %s", ip.String())
				return ip.String(), nil
			}

			logger.Warnf("UDP local address is not IPv4: %v", addr.IP)
		} else {
			logger.Warnf("Failed to cast UDP local address")
		}
	} else {
		logger.Warnf("UDP dial failed, falling back to interface scan: %v", err)
	}

	return getInterfaceIP()
}

func getInterfaceIP() (string, error) {
	logger.Infof("Scanning network interfaces for LAN IP...")

	ifaces, err := net.Interfaces()
	if err != nil {
		logger.Errorf("Failed to list network interfaces: %v", err)
		return "", err
	}

	logger.Debugf("Found %d network interfaces", len(ifaces))

	for _, iface := range ifaces {
		logger.Debugf("Checking interface: %s (flags: %v)", iface.Name, iface.Flags)
		if iface.Flags&net.FlagUp == 0 {
			logger.Debugf("Skipping %s: interface down", iface.Name)
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			logger.Debugf("Skipping %s: loopback interface", iface.Name)
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			logger.Warnf("Failed to get addresses for %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}

			if isPrivateIPv4(ip4) {
				logger.Infof("Detected private LAN IP via interface %s: %s", iface.Name, ip4.String())
				return ip4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no active private LAN network interface found")
}

var privateRanges = []*net.IPNet{
	{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
	{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
	{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)},
	{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)},  // CGNAT
	{IP: net.IP{169, 254, 0, 0}, Mask: net.CIDRMask(16, 32)}, // Link-local
}

func isPrivateIPv4(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}
