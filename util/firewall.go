package util

func SetFirewallRule(port, oldPort int) error {
	return setFirewallRule(port, oldPort)
}

func CheckIfRuleExist() (bool, error) {
	return checkIfRuleExistOS()
}
