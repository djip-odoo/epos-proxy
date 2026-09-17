package bluetooth

import (
	"bytes"
	"testing"
)

func TestParseMACToBytes(t *testing.T) {
	tests := []struct {
		name      string
		mac       string
		want      [6]byte
		expectErr bool
	}{
		{
			name:      "valid standard MAC",
			mac:       "11:22:33:44:55:66",
			want:      [6]byte{0x66, 0x55, 0x44, 0x33, 0x22, 0x11}, // reversed little-endian
			expectErr: false,
		},
		{
			name:      "valid lowercase MAC",
			mac:       "aa:bb:cc:dd:ee:ff",
			want:      [6]byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa},
			expectErr: false,
		},
		{
			name:      "invalid octet count",
			mac:       "11:22:33:44:55",
			want:      [6]byte{},
			expectErr: true,
		},
		{
			name:      "invalid hex characters",
			mac:       "11:22:33:44:55:ZZ",
			want:      [6]byte{},
			expectErr: true,
		},
		{
			name:      "empty string",
			mac:       "",
			want:      [6]byte{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMACToBytes(tt.mac)
			if (err != nil) != tt.expectErr {
				t.Fatalf("ParseMACToBytes(%q) error = %v, expectErr = %v", tt.mac, err, tt.expectErr)
			}
			if !tt.expectErr && !bytes.Equal(got[:], tt.want[:]) {
				t.Errorf("ParseMACToBytes(%q) = %v, want %v", tt.mac, got, tt.want)
			}
		})
	}
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "aa-bb-cc-dd-ee-ff",
			want:  "AA:BB:CC:DD:EE:FF",
		},
		{
			input: "  11:22:33:44:55:66  ",
			want:  "11:22:33:44:55:66",
		},
		{
			input: "00001800-0000-1000-8000-00805f9b34fb",
			want:  "00001800-0000-1000-8000-00805F9B34FB",
		},
		{
			input: "  e2c56db5-dffb-48d2-b060-d0f5a71096e0  ",
			want:  "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeAddress(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeAddress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	valid := []string{
		"AA:BB:CC:DD:EE:FF",
		"aa:bb:cc:dd:ee:ff",
		"AA-BB-CC-DD-EE-FF",
		"00:11:22:33:44:55",
		"00001800-0000-1000-8000-00805f9b34fb",
		"E2C56DB5-DFFB-48D2-B060-D0F5A71096E0",
	}

	for _, addr := range valid {
		t.Run("valid_"+addr, func(t *testing.T) {
			if err := ValidateAddress(addr); err != nil {
				t.Errorf("ValidateAddress(%q) unexpected error: %v", addr, err)
			}
		})
	}

	invalid := []string{
		"",
		"   ",
		"invalid-mac",
		"11:22:33:44:55",
		"11:22:33:44:55:66:77",
		"GG:HH:II:JJ:KK:LL",
		"00001800-0000-1000-8000-00805f9b34f",
	}

	for _, addr := range invalid {
		t.Run("invalid_"+addr, func(t *testing.T) {
			if err := ValidateAddress(addr); err == nil {
				t.Errorf("ValidateAddress(%q) expected error, got nil", addr)
			}
		})
	}
}
