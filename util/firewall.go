package util

// FirewallStatus represents the current status of the firewall for the given port
type FirewallStatus string

const (
	FirewallStatusAllowed FirewallStatus = "Allowed"
	FirewallStatusBlocked FirewallStatus = "Blocked/Unknown"
)

// AllowPortThroughFirewall prompts for elevation and allows the port through the firewall
func AllowPortThroughFirewall(port int) error {
	return allowPortOS(port)
}

// BlockPortThroughFirewall prompts for elevation and removes the rule allowing the port
func BlockPortThroughFirewall(port int) error {
	return blockPortOS(port)
}
