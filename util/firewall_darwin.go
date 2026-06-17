package util

import (
	"epos-proxy/logger"
)

func allowPortOS(port int) error {
	logger.Infof(
		"macOS uses an application-based firewall; no explicit port rule configuration is required",
	)
	return nil
}

func blockPortOS(port int) error {
	logger.Infof(
		"macOS uses an application-based firewall; no explicit port rule removal is required",
	)
	return nil
}
