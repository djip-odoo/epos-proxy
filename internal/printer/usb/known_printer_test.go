package usb

import (
	"testing"

	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"
)

func TestGetPrinterType(t *testing.T) {
	tests := []struct {
		vidPid   string
		expected printer.Type
	}{
		// Known receipt printers
		{"2aaf:6015", printer.TypeReceipt},
		{"2AAF:6015", printer.TypeReceipt}, // case-insensitive
		{"04b8:0e32", printer.TypeReceipt},
		{"04B8:0202", printer.TypeReceipt},
		{"04b8:0e27", printer.TypeReceipt},
		{"0483:5720", printer.TypeReceipt},
		{"2d84:c7c8", printer.TypeReceipt},
		{"4b43:3830", printer.TypeReceipt},

		// Known label printers
		{"0a5f:0187", printer.TypeLabel},
		{"0A5F:0187", printer.TypeLabel}, // case-insensitive
		{"195f:0001", printer.TypeLabel},

		// Unknown VID:PID -> defaults to TypeReceipt
		{"1234:5678", printer.TypeReceipt},
		{"", printer.TypeReceipt},
	}

	for _, tc := range tests {
		got := getPrinterType(tc.vidPid)
		testutil.ExpectedEqual(t, got, tc.expected)
	}
}

func TestIsKnownPrinter(t *testing.T) {
	// Epson printer 04b8:0202
	epsonDesc := testutil.MockEpsonPrinterDesc()
	testutil.ExpectedTrue(t, isKnownPrinter(epsonDesc))

	// Zebra printer 0a5f:0187
	zebraDesc := testutil.MockZebraPrinterDesc()
	testutil.ExpectedTrue(t, isKnownPrinter(zebraDesc))

	// Non-printer mass storage 1234:5678
	storageDesc := testutil.MockMassStorageDesc()
	testutil.ExpectedFalse(t, isKnownPrinter(storageDesc))
}
