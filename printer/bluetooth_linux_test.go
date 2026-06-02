//go:build linux

package printer

import (
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// findFreeRFCOMMIndex
// ---------------------------------------------------------------------------

func TestFindFreeRFCOMMIndex(t *testing.T) {
	tests := []struct {
		name     string
		existing map[int]string
		want     int
	}{
		{
			name:     "empty — returns 0",
			existing: map[int]string{},
			want:     0,
		},
		{
			name:     "0 taken — returns 1",
			existing: map[int]string{0: "AA:BB:CC:DD:EE:FF"},
			want:     1,
		},
		{
			name:     "0 and 1 taken — returns 2",
			existing: map[int]string{0: "AA:BB:CC:DD:EE:FF", 1: "11:22:33:44:55:66"},
			want:     2,
		},
		{
			name: "gap at 3 — returns 3",
			existing: map[int]string{
				0: "AA:BB:CC:DD:EE:01",
				1: "AA:BB:CC:DD:EE:02",
				2: "AA:BB:CC:DD:EE:03",
				4: "AA:BB:CC:DD:EE:05",
			},
			want: 3,
		},
		{
			name: "all taken — returns -1",
			existing: func() map[int]string {
				m := make(map[int]string, 32)
				for i := 0; i <= 31; i++ {
					m[i] = "AA:BB:CC:DD:EE:FF"
				}
				return m
			}(),
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findFreeRFCOMMIndex(tt.existing)
			if got != tt.want {
				t.Errorf("findFreeRFCOMMIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseRFCOMMOutput (table-driven test for listRFCOMMBindings parsing logic)
// ---------------------------------------------------------------------------

// parseRFCOMMLinesForTest is extracted from listRFCOMMBindings for unit testing
// without requiring the rfcomm binary.
func parseRFCOMMLinesForTest(output string) map[int]string {
	bindings := make(map[int]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		devName := strings.TrimSpace(line[:colonIdx])
		if !strings.HasPrefix(devName, "rfcomm") {
			continue
		}
		idxStr := strings.TrimPrefix(devName, "rfcomm")
		var idx int
		if n, err := parseInt(idxStr); err != nil {
			continue
		} else {
			idx = n
		}
		rest := strings.TrimSpace(line[colonIdx+1:])
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			continue
		}
		bindings[idx] = NormalizeMAC(fields[0])
	}
	return bindings
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func TestParseRFCOMMLines(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[int]string
	}{
		{
			name:   "empty output",
			output: "",
			want:   map[int]string{},
		},
		{
			name:   "single binding",
			output: "rfcomm0: AA:BB:CC:DD:EE:FF channel 1 clean\n",
			want:   map[int]string{0: "AA:BB:CC:DD:EE:FF"},
		},
		{
			name: "multiple bindings",
			output: `rfcomm0: AA:BB:CC:DD:EE:FF channel 1 clean
rfcomm1: 11:22:33:44:55:66 channel 3 clean
`,
			want: map[int]string{
				0: "AA:BB:CC:DD:EE:FF",
				1: "11:22:33:44:55:66",
			},
		},
		{
			name:   "lowercase mac normalised",
			output: "rfcomm2: aa:bb:cc:dd:ee:ff channel 2 clean\n",
			want:   map[int]string{2: "AA:BB:CC:DD:EE:FF"},
		},
		{
			name:   "non-rfcomm line ignored",
			output: "hci0: some text\nrfcomm0: AA:BB:CC:DD:EE:FF channel 1 clean\n",
			want:   map[int]string{0: "AA:BB:CC:DD:EE:FF"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRFCOMMLinesForTest(tt.output)
			if len(got) != len(tt.want) {
				t.Errorf("got %d entries, want %d: %v", len(got), len(tt.want), got)
				return
			}
			for idx, mac := range tt.want {
				if got[idx] != mac {
					t.Errorf("index %d: got MAC %q, want %q", idx, got[idx], mac)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rfcommCache
// ---------------------------------------------------------------------------

func TestRFCOMMCache(t *testing.T) {
	c := &rfcommCache{entries: make(map[string]*rfcommBinding)}

	// Miss
	if _, ok := c.get("AA:BB:CC:DD:EE:FF"); ok {
		t.Fatal("expected cache miss, got hit")
	}

	// Set and hit
	b := &rfcommBinding{DevPath: "/dev/rfcomm0", Channel: 1, Index: 0}
	c.set("AA:BB:CC:DD:EE:FF", b)

	got, ok := c.get("AA:BB:CC:DD:EE:FF")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if got.DevPath != "/dev/rfcomm0" || got.Channel != 1 {
		t.Errorf("unexpected cached value: %+v", got)
	}

	// Different key is still a miss
	if _, ok := c.get("11:22:33:44:55:66"); ok {
		t.Fatal("unexpected cache hit for different MAC")
	}
}

// ---------------------------------------------------------------------------
// sdpDiscoverLinux (parser only, no subprocess)
// ---------------------------------------------------------------------------

// parseSDP mirrors the parsing logic of sdpDiscoverLinux so we can test it
// without spawning sdptool.
func parseSDP(output string) (int, bool) {
	lines := strings.Split(output, "\n")
	inSerialPort := false
	uuidSerial := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "service name:") {
			if strings.Contains(lower, "serial port") {
				inSerialPort = true
			} else {
				inSerialPort = false
			}
			uuidSerial = false
			continue
		}

		if strings.HasPrefix(lower, "uuid") &&
			(strings.Contains(lower, "0x1101") ||
				strings.Contains(lower, "00001101") ||
				strings.Contains(lower, "serial port")) {
			uuidSerial = true
			continue
		}

		if strings.HasPrefix(lower, "channel:") && (inSerialPort || uuidSerial) {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				n, err := strconv.Atoi(parts[1])
				if err == nil && n > 0 {
					return n, true
				}
			}
		}
	}
	return 0, false
}

func TestParseSDP(t *testing.T) {
	tests := []struct {
		name   string
		output string
		wantCh int
		wantOK bool
	}{
		{
			name: "mode-A: named Serial Port service",
			output: `Browsing 00:11:22:33:44:55 ...
Service Name: Serial Port
Service RecHandle: 0x10000
Service Class ID List:
  "Serial Port" (0x1101)
Protocol Descriptor List:
  "L2CAP" (0x0100)
  "RFCOMM" (0x0003)
    Channel: 1
`,
			wantCh: 1,
			wantOK: true,
		},
		{
			name: "mode-B: UUID 0x1101 without service name header",
			output: `Browsing 00:11:22:33:44:55 ...
Service RecHandle: 0x10002
Service Class ID List:
  UUID: 0x1101
Protocol Descriptor List:
  "L2CAP" (0x0100)
  "RFCOMM" (0x0003)
    Channel: 3
`,
			wantCh: 3,
			wantOK: true,
		},
		{
			name: "mode-B: UUID 00001101 (128-bit style)",
			output: `UUID 128: 00001101-0000-1000-8000-00805f9b34fb
Channel: 5
`,
			wantCh: 5,
			wantOK: true,
		},
		{
			name:   "no serial port record",
			output: `Service Name: Headset\nChannel: 2\n`,
			wantCh: 0,
			wantOK: false,
		},
		{
			name:   "empty output",
			output: "",
			wantCh: 0,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, ok := parseSDP(tt.output)
			if ok != tt.wantOK || ch != tt.wantCh {
				t.Errorf("parseSDP() = (%d, %v), want (%d, %v)", ch, ok, tt.wantCh, tt.wantOK)
			}
		})
	}
}
