//go:build windows

package bluetooth

import (
	"encoding/binary"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestWindows_BTHAddressConversion(t *testing.T) {
	tests := []struct {
		name    string
		mac     string
		wantHex uint64
		wantErr bool
	}{
		{
			name:    "valid MAC",
			mac:     "00:11:22:33:44:55",
			wantHex: 0x001122334455,
			wantErr: false,
		},
		{
			name:    "valid MAC lowercase",
			mac:     "aa:bb:cc:dd:ee:ff",
			wantHex: 0xAABBCCDDEEFF,
			wantErr: false,
		},
		{
			name:    "invalid MAC length",
			mac:     "11:22:33:44:55",
			wantErr: true,
		},
		{
			name:    "invalid MAC char",
			mac:     "11:22:33:44:55:ZZ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := macToWindowsBTHAddr(tt.mac)
			if tt.wantErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedEqual(t, got, tt.wantHex)

				// Roundtrip test
				back := btAddrToMAC(got)
				normExpected := normalizeAddress(tt.mac)
				testutil.ExpectedEqual(t, back, normExpected)
			}
		})
	}
}

func TestWindows_MakeSockaddrBTH(t *testing.T) {
	const addr uint64 = 0x001122334455
	const channel uint32 = 1

	sa := makeSockaddrBTH(addr, channel)

	family := binary.LittleEndian.Uint16(sa[0:2])
	testutil.ExpectedEqual(t, family, afBTH)

	gotAddr := binary.LittleEndian.Uint64(sa[2:10])
	testutil.ExpectedEqual(t, gotAddr, addr)

	gotCh := binary.LittleEndian.Uint32(sa[26:30])
	testutil.ExpectedEqual(t, gotCh, channel)
}

func TestWindows_CheckDependencies(t *testing.T) {
	deps := checkDependencies()
	testutil.ExpectedLen(t, deps, 0)
}
