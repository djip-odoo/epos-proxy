//go:build !windows

package printer

import (
	"os"
)

// fdToFile wraps a raw file descriptor as an *os.File. Used to bridge the
// raw RFCOMM fd into a net.Conn via net.FileConn.
func fdToFile(fd int, name string) (*os.File, error) {
	return os.NewFile(uintptr(fd), name), nil
}
