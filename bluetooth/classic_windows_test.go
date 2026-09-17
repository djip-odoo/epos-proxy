//go:build windows

package bluetooth

import (
	"encoding/binary"
	"testing"
)

func TestClassicTransport_Windows(t *testing.T) {
	trans := &ClassicTransport{}
	if trans.Name() != "Classic" {
		t.Errorf("expected transport name 'Classic', got %q", trans.Name())
	}
}

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
			if (err != nil) != tt.wantErr {
				t.Fatalf("macToWindowsBTHAddr(%q) err = %v, wantErr = %v", tt.mac, err, tt.wantErr)
			}
			if !tt.wantErr {
				if got != tt.wantHex {
					t.Errorf("macToWindowsBTHAddr(%q) = 0x%X, want 0x%X", tt.mac, got, tt.wantHex)
				}
				// Roundtrip test
				back := btAddrToMAC(got)
				normExpected := NormalizeAddress(tt.mac)
				if back != normExpected {
					t.Errorf("btAddrToMAC(0x%X) = %q, want %q", got, back, normExpected)
				}
			}
		})
	}
}

func TestWindows_MakeSockaddrBTH(t *testing.T) {
	const addr uint64 = 0x001122334455
	const channel uint32 = 1

	sa := makeSockaddrBTH(addr, channel)

	family := binary.LittleEndian.Uint16(sa[0:2])
	if family != afBTH {
		t.Errorf("expected family afBTH (%d), got %d", afBTH, family)
	}

	gotAddr := binary.LittleEndian.Uint64(sa[2:10])
	if gotAddr != addr {
		t.Errorf("expected address 0x%X, got 0x%X", addr, gotAddr)
	}

	gotCh := binary.LittleEndian.Uint32(sa[26:30])
	if gotCh != channel {
		t.Errorf("expected channel %d, got %d", channel, gotCh)
	}
}

func TestWindows_CheckDependencies(t *testing.T) {
	deps := CheckDependencies()
	if len(deps) != 0 {
		t.Errorf("expected empty dependencies on windows, got %d", len(deps))
	}
}
