package printer

import (
	"os"
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

// TestMain seeds the registry once, mirroring the startup load in app.go.
func TestMain(m *testing.M) {
	initKnownPrinterRegistry(config.DefaultKnownPrinters())
	os.Exit(m.Run())
}

func TestGetPrinterType(t *testing.T) {
	tests := []struct {
		vidPid   string
		expected Type
	}{
		// Known receipt printers
		{"2aaf:6015", TypeReceipt},
		{"2AAF:6015", TypeReceipt}, // case-insensitive
		{"04b8:0e32", TypeReceipt},
		{"04B8:0202", TypeReceipt},
		{"04b8:0e27", TypeReceipt},
		{"0483:5720", TypeReceipt},
		{"2d84:c7c8", TypeReceipt},
		{"4b43:3830", TypeReceipt},

		// Known label printers
		{"0a5f:0187", TypeLabel},
		{"0A5F:0187", TypeLabel}, // case-insensitive
		{"195f:0001", TypeLabel},

		// Unknown VID:PID -> defaults to TypeReceipt
		{"1234:5678", TypeReceipt},
		{"", TypeReceipt},
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

func TestInitKnownPrinterRegistry(t *testing.T) {
	// Restore defaults afterwards so other tests are unaffected.
	defer initKnownPrinterRegistry(config.DefaultKnownPrinters())

	// Custom registry replaces the built-in defaults.
	initKnownPrinterRegistry([]config.KnownPrinter{
		{VID: "1234", PID: "5678", Name: "Custom", Type: "label"},
		{VID: "abcd", PID: "ef01", Name: "Typo", Type: "reciept"},
	})

	testutil.ExpectedEqual(t, getPrinterType("1234:5678"), TypeLabel)
	testutil.ExpectedEqual(t, getPrinterType("1234:5679"), TypeReceipt) // unknown -> receipt
	testutil.ExpectedTrue(t, isKnownPrinter(testutil.MockPrinterDesc(0x1234, 0x5678)))

	// An unrecognised type stays registered but defaults to receipt.
	testutil.ExpectedEqual(t, getPrinterType("abcd:ef01"), TypeReceipt)
	testutil.ExpectedTrue(t, isKnownPrinter(testutil.MockPrinterDesc(0xabcd, 0xef01)))
	testutil.ExpectedEqual(t, getKnownPrinterName("abcd:ef01"), "Typo")

	// The built-in defaults are gone.
	testutil.ExpectedEqual(t, getPrinterType("04b8:0202"), TypeReceipt) // falls back, not known
	testutil.ExpectedFalse(t, isKnownPrinter(testutil.MockEpsonPrinterDesc()))
}

func TestKnownPrinterName(t *testing.T) {
	name := getKnownPrinterName("04B8:0E32")
	testutil.ExpectedEqual(t, name, "Epson thermal")

	testutil.ExpectedEqual(t, getKnownPrinterName("ABCD:1234"), "")
}
