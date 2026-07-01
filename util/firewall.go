package util

// AllowPortThroughFirewall prompts for elevation and allows the port through the firewall
func AllowPortThroughFirewall(port int) error {
	return allowPortOS(port)
}

// AllowApplicationThroughFirewall prompts for elevation and allows the application through the firewall
func AllowApplicationThroughFirewall() error {
	return allowApplicationOS()
}

// BlockPortThroughFirewall prompts for elevation and removes the rule allowing the port
func BlockPortThroughFirewall(port int) error {
	return blockPortOS(port)
}
