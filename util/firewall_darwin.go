package util

import (
	"epos-proxy/logger"
)

func allowPortOS(port int) error {
	logger.Infof("macOS uses an application firewall; no explicit port rules required. The OS should prompt automatically if needed.")
	return nil
}

func blockPortOS(port int) error {
	logger.Infof("macOS uses an application firewall; no explicit port rules required.")
	return nil
}
