package utils

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/features"
	v1 "k8s.io/api/core/v1"
)

func FormatResourceList(resources v1.ResourceList) string {
	resourceStrings := make([]string, 0, len(resources))
	for key, value := range resources {
		resourceStrings = append(resourceStrings, fmt.Sprintf("%v=%v", key, value.String()))
	}
	// sort the results for consistent log output
	sort.Strings(resourceStrings)
	return strings.Join(resourceStrings, ",")
}

// IsPlainIPV6 will return true if the given address is a plain IPV6 address and not encapsulated or similar.
func IsPlainIPV6(address string) bool {
	ipAddr, err := netip.ParseAddr(address)
	if err != nil {
		return false
	}
	return ipAddr.Is6()
}

// IsMCMServerOnly identifies when a Rancher instance is configured as an MCM server and not an MCM Agent
func IsMCMServerOnly() bool {
	return !features.MCMAgent.Enabled() && features.MCM.Enabled()
}

// IsPrivateHost resolves the given hostname and returns true if any of
// its addresses are loopback, private, link-local, or unspecified.
// Returns true on resolution failure (deny by default).
func IsPrivateHost(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return true
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}
