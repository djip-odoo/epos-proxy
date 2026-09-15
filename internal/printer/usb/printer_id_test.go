package usb

import (
	"errors"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestEncodePrinterID(t *testing.T) {
	// Case 1: Serial and VidPid provided -> encodes both vp:<vidpid> and s:<serial>
	pWithSerial := &LibUsbPrinter{
		Serial: "SN123456",
		VidPid: "04B8:0202",
		Path:   "1.2.3",
	}
	encoded, err := encodeID(pWithSerial)
	testutil.ExpectedNoError(t, err)

	decoded, err := decodeID(encoded)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, decoded.Serial, "SN123456")
	testutil.ExpectedEqual(t, decoded.VidPid, "04B8:0202")
	testutil.ExpectedEqual(t, decoded.Path, "")

	// Case 2: Only serial provided (no VidPid, no Path)
	pOnlySerial := &LibUsbPrinter{Serial: "SN987654"}
	encodedOnlySerial, err := encodeID(pOnlySerial)
	testutil.ExpectedNoError(t, err)

	decodedOnlySerial, err := decodeID(encodedOnlySerial)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, decodedOnlySerial.Serial, "SN987654")
	testutil.ExpectedEqual(t, decodedOnlySerial.VidPid, "")
	testutil.ExpectedEqual(t, decodedOnlySerial.Path, "")

	// Case 3: No serial, but VidPid and Path provided
	pNoSerial := &LibUsbPrinter{
		VidPid: "04B8:0202",
		Path:   "1.2.3",
	}
	encoded2, err := encodeID(pNoSerial)
	testutil.ExpectedNoError(t, err)

	decoded2, err := decodeID(encoded2)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, decoded2.Serial, "")
	testutil.ExpectedEqual(t, decoded2.VidPid, "04B8:0202")
	testutil.ExpectedEqual(t, decoded2.Path, "1.2.3")

	// Case 4: Completely empty LibUsbPrinter -> should return error
	pEmpty := &LibUsbPrinter{}
	_, err = encodeID(pEmpty)
	testutil.ExpectedError(t, err)
}

func TestDecodePrinterID_Invalid(t *testing.T) {
	// Invalid base64
	_, err := decodeID("not-valid-base64!!!")
	testutil.ExpectedTrue(t, errors.Is(err, ErrInvalidPrinterID))

	// Valid base64 but empty payload
	_, err = decodeID("")
	testutil.ExpectedTrue(t, errors.Is(err, ErrInvalidPrinterID))
}
