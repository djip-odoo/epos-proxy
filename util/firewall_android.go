package util

import (
	"errors"
)

var ErrAuthCancelled = errors.New("authentication cancelled")

func setFirewallRule(port, oldPort int) error {
	return nil
}

func checkIfRuleExistOS() (bool, error) {
	return false, nil
}
