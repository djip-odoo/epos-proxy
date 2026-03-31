package printer

import (
	"strconv"
	"strings"

	"github.com/google/gousb"
)

func PathToString(desc *gousb.DeviceDesc) string {
	parts := make([]string, 0, len(desc.Path)+1)

	// Add bus first
	parts = append(parts, strconv.Itoa(desc.Bus))

	// Convert each path element
	for _, p := range desc.Path {
		parts = append(parts, strconv.Itoa(p))
	}

	return strings.Join(parts, ".")
}
