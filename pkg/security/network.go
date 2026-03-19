package security

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
)

// BlockedNetworks contains the CIDR ranges that are never allowed for outbound
// HTTP targets.
var BlockedNetworks []*net.IPNet

func init() {
	cidrs := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid blocked CIDR %q: %v", cidr, err))
		}
		BlockedNetworks = append(BlockedNetworks, network)
	}
}

var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func ValidateURLTarget(rawURL string) error {
	return validateURL(rawURL)
}

func ValidateResolvedURL(rawURL string) error {
	return validateURL(rawURL)
}

func ContainsInternalURL(text string) error {
	for _, rawURL := range urlPattern.FindAllString(text, -1) {
		if err := ValidateURLTarget(rawURL); err != nil {
			return fmt.Errorf("internal URL %q rejected: %w", rawURL, err)
		}
	}
	return nil
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked IP %s", ip.String())
		}
		return nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("resolve host %q: no addresses returned", host)
	}

	for _, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return fmt.Errorf("host %q resolves to blocked IP %s", host, addr.IP.String())
		}
	}

	return nil
}

func isBlockedIP(ip net.IP) bool {
	normalized := ip
	if v4 := ip.To4(); v4 != nil {
		normalized = v4
	}
	for _, network := range BlockedNetworks {
		if network.Contains(normalized) {
			return true
		}
	}
	return false
}
