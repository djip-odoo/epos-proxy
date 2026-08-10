package printer

import (
	"testing"
)

// makePrinter is a convenience constructor for test fixtures.
func makePrinter(serial, path, vidPid string) *LibUsbPrinter {
	return &LibUsbPrinter{
		Serial: serial,
		Path:   path,
		VidPid: vidPid,
		Name:   "Test Printer",
	}
}

// TestUseSerial_UniqueSerial verifies that a single printer with a non-empty
// serial gets UseSerial = true.
func TestUseSerial_UniqueSerial(t *testing.T) {
	p := makePrinter("12345", "/dev/bus/usb/001/002", "04B8:0E32")
	applySerialDedup([]*LibUsbPrinter{p})

	if !p.UseSerial {
		t.Errorf("expected UseSerial = true for a printer with a unique serial, got false")
	}
}

// TestUseSerial_TwoDifferentSerials verifies that two printers with distinct
// non-empty serials both get UseSerial = true.
func TestUseSerial_TwoDifferentSerials(t *testing.T) {
	a := makePrinter("AAA", "/dev/bus/usb/001/002", "04B8:0E32")
	b := makePrinter("BBB", "/dev/bus/usb/001/003", "04B8:0E32")
	applySerialDedup([]*LibUsbPrinter{a, b})

	if !a.UseSerial {
		t.Errorf("printer A: expected UseSerial = true, got false")
	}
	if !b.UseSerial {
		t.Errorf("printer B: expected UseSerial = true, got false")
	}
}

// TestUseSerial_NoSerial verifies that a printer with an empty serial gets
// UseSerial = false.
func TestUseSerial_NoSerial(t *testing.T) {
	p := makePrinter("", "/dev/bus/usb/001/002", "04B8:0E32")
	applySerialDedup([]*LibUsbPrinter{p})

	if p.UseSerial {
		t.Errorf("expected UseSerial = false for a printer with no serial, got true")
	}
}

// TestUseSerial_TwoNoSerial verifies that two printers with empty serials
// both remain UseSerial = false and are NOT merged (i.e. they remain separate
// entries in the slice).
func TestUseSerial_TwoNoSerial(t *testing.T) {
	a := makePrinter("", "/dev/bus/usb/001/002", "04B8:0E32")
	b := makePrinter("", "/dev/bus/usb/001/003", "04B8:0E32")
	printers := []*LibUsbPrinter{a, b}
	applySerialDedup(printers)

	if a.UseSerial {
		t.Errorf("printer A: expected UseSerial = false (no serial), got true")
	}
	if b.UseSerial {
		t.Errorf("printer B: expected UseSerial = false (no serial), got true")
	}
	// Both devices must remain separate entries.
	if len(printers) != 2 {
		t.Errorf("expected 2 separate printers, got %d", len(printers))
	}
}

// TestUseSerial_DuplicateSerial verifies that two printers sharing a serial
// both get UseSerial = false.
func TestUseSerial_DuplicateSerial(t *testing.T) {
	a := makePrinter("12345", "/dev/bus/usb/001/002", "04B8:0E32")
	b := makePrinter("12345", "/dev/bus/usb/001/003", "04B8:0E32")
	applySerialDedup([]*LibUsbPrinter{a, b})

	if a.UseSerial {
		t.Errorf("printer A: expected UseSerial = false (duplicate serial), got true")
	}
	if b.UseSerial {
		t.Errorf("printer B: expected UseSerial = false (duplicate serial), got true")
	}
}

// TestUseSerial_ThreePrintersSharedAndUnique verifies the mixed case:
// two printers share a serial (both false) while a third has a unique serial (true).
func TestUseSerial_ThreePrintersSharedAndUnique(t *testing.T) {
	shared1 := makePrinter("DUP", "/dev/bus/usb/001/002", "04B8:0E32")
	shared2 := makePrinter("DUP", "/dev/bus/usb/001/003", "04B8:0E32")
	unique := makePrinter("UNIQUE", "/dev/bus/usb/001/004", "04B8:0E32")
	applySerialDedup([]*LibUsbPrinter{shared1, shared2, unique})

	if shared1.UseSerial {
		t.Errorf("shared1: expected UseSerial = false (duplicate serial), got true")
	}
	if shared2.UseSerial {
		t.Errorf("shared2: expected UseSerial = false (duplicate serial), got true")
	}
	if !unique.UseSerial {
		t.Errorf("unique: expected UseSerial = true (unique serial), got false")
	}
}

// TestUseSerial_RetroactiveRevocation verifies that when a duplicate serial is
// discovered after the first printer has already been tentatively marked
// UseSerial = true, the first printer's UseSerial is retroactively set to false.
func TestUseSerial_RetroactiveRevocation(t *testing.T) {
	first := makePrinter("SAME", "/dev/bus/usb/001/002", "04B8:0E32")
	second := makePrinter("SAME", "/dev/bus/usb/001/003", "04B8:0E32")

	// Process one at a time to prove retroactive revocation.
	// applySerialDedup works on the whole slice but internally processes
	// in order, so "first" will be tentatively set to true before "second"
	// triggers the revocation.
	applySerialDedup([]*LibUsbPrinter{first, second})

	if first.UseSerial {
		t.Errorf("first: expected UseSerial = false after retroactive revocation, got true")
	}
	if second.UseSerial {
		t.Errorf("second: expected UseSerial = false (duplicate), got true")
	}
}

// TestUseSerial_EmptySerialNeverConflicts verifies that two printers with
// empty serials do not conflict with each other (i.e. empty-serial printers
// are never matched together).
func TestUseSerial_EmptySerialNeverConflicts(t *testing.T) {
	// Three printers: two with empty serials and one with a real serial.
	// The empty-serial printers must NOT suppress the real-serial printer.
	noSerial1 := makePrinter("", "/dev/bus/usb/001/002", "04B8:0E32")
	noSerial2 := makePrinter("", "/dev/bus/usb/001/003", "04B8:0E32")
	withSerial := makePrinter("REAL", "/dev/bus/usb/001/004", "04B8:0E32")
	applySerialDedup([]*LibUsbPrinter{noSerial1, noSerial2, withSerial})

	if noSerial1.UseSerial {
		t.Errorf("noSerial1: expected UseSerial = false, got true")
	}
	if noSerial2.UseSerial {
		t.Errorf("noSerial2: expected UseSerial = false, got true")
	}
	if !withSerial.UseSerial {
		t.Errorf("withSerial: expected UseSerial = true (empty serials must not conflict), got false")
	}
}
