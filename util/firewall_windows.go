package util

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"printer-manager/logger"
)

const ruleNamePrefix = "EPOS Proxy LAN Port "

var (
	shell32       = windows.NewLazySystemDLL("shell32.dll")
	shellExecuteW = shell32.NewProc("ShellExecuteW")
)

func allowPortOS(port int) error {
	ruleName := fmt.Sprintf("%s%d", ruleNamePrefix, port)

	logger.Infof(
		"Attempting to allow port %d through Windows firewall",
		port,
	)

	args := fmt.Sprintf(
		`advfirewall firewall add rule name="%s" dir=in action=allow protocol=TCP localport=%d profile=private`,
		ruleName,
		port,
	)

	return runNetshElevated(args)
}

func blockPortOS(port int) error {
	ruleName := fmt.Sprintf("%s%d", ruleNamePrefix, port)

	logger.Infof(
		"Attempting to block port %d through Windows firewall",
		port,
	)

	args := fmt.Sprintf(
		`advfirewall firewall delete rule name="%s"`,
		ruleName,
	)

	return runNetshElevated(args)
}

func runNetshElevated(args string) error {
	verb := syscall.StringToUTF16Ptr("runas")
	exe := syscall.StringToUTF16Ptr("netsh")
	params := syscall.StringToUTF16Ptr(args)

	ret, _, err := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exe)),
		uintptr(unsafe.Pointer(params)),
		0,
		windows.SW_HIDE,
	)

	// Per ShellExecute docs:
	// return value > 32 means success
	if ret <= 32 {
		return fmt.Errorf(
			"failed to execute elevated netsh command: %v",
			err,
		)
	}

	return nil
}
