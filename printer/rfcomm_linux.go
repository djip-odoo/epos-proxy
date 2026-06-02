//go:build linux

package printer

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"epos-proxy/logger"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// rfcommBinding records the state of a bound RFCOMM kernel device.
type rfcommBinding struct {
	DevPath string // e.g. "/dev/rfcomm0"
	Channel int    // RFCOMM channel number
	Index   int    // numeric index (0 = rfcomm0, 1 = rfcomm1, …)
}

// rfcommCache is a process-level cache mapping normalised MAC → binding.
// It avoids repeated SDP scans and rfcomm bind calls within a single run.
type rfcommCache struct {
	mu      sync.Mutex
	entries map[string]*rfcommBinding
}

var globalRFCOMMCache = &rfcommCache{
	entries: make(map[string]*rfcommBinding),
}

func (c *rfcommCache) get(mac string) (*rfcommBinding, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.entries[mac]
	return b, ok
}

func (c *rfcommCache) set(mac string, b *rfcommBinding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[mac] = b
}

// ---------------------------------------------------------------------------
// List existing RFCOMM bindings
// ---------------------------------------------------------------------------

// listRFCOMMBindings runs `rfcomm -a` and returns a map of
// index → normalised-MAC for all currently bound RFCOMM devices.
// Returns an empty map (not an error) when no devices are bound.
func listRFCOMMBindings() (map[int]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rfcomm", "-a").CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("rfcomm -a timed out")
	}
	// rfcomm exits 0 even with no output; non-zero usually means the binary
	// is missing or BlueZ is not running — surface that.
	if err != nil {
		return nil, fmt.Errorf("rfcomm -a failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	bindings := make(map[int]string)
	// Example line:
	//   rfcomm0: AA:BB:CC:DD:EE:FF channel 1 clean
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Extract device name like "rfcomm0:"
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		devName := strings.TrimSpace(line[:colonIdx]) // "rfcomm0"
		if !strings.HasPrefix(devName, "rfcomm") {
			continue
		}
		idxStr := strings.TrimPrefix(devName, "rfcomm")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			continue
		}

		// Rest of the line after the colon: " AA:BB:CC:DD:EE:FF channel 1 clean"
		rest := strings.TrimSpace(line[colonIdx+1:])
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			continue
		}
		mac := NormalizeMAC(fields[0])
		bindings[idx] = mac
	}

	logger.Debugf("BT/RFCOMM: listRFCOMMBindings found %d bound device(s): %v", len(bindings), bindings)
	return bindings, nil
}

// ---------------------------------------------------------------------------
// Find existing device for a MAC
// ---------------------------------------------------------------------------

// findExistingRFCOMMDevice searches the current RFCOMM bindings for one that
// matches mac.  If found it also checks whether the /dev node exists.
// Returns (binding, true) on a match, (nil, false) otherwise.
func findExistingRFCOMMDevice(mac string) (*rfcommBinding, bool) {
	mac = NormalizeMAC(mac)

	bindings, err := listRFCOMMBindings()
	if err != nil {
		// rfcomm binary may be missing; treat as no bindings found.
		logger.Warnf("BT/RFCOMM: could not list RFCOMM bindings: %v", err)
		return nil, false
	}

	for idx, boundMAC := range bindings {
		if boundMAC == mac {
			devPath := fmt.Sprintf("/dev/rfcomm%d", idx)
			if _, err := os.Stat(devPath); err != nil {
				// Device node doesn't exist yet (kernel may still be creating it).
				logger.Warnf("BT/RFCOMM: binding for %s found at index %d but %s does not exist: %v",
					mac, idx, devPath, err)
			}
			b := &rfcommBinding{
				DevPath: devPath,
				Index:   idx,
				// Channel not available from rfcomm -a output reliably; caller sets it from SDP/cache.
				Channel: 0,
			}
			logger.Infof("BT/RFCOMM: existing binding found for %s → %s", mac, devPath)
			return b, true
		}
	}

	logger.Debugf("BT/RFCOMM: no existing RFCOMM binding for %s", mac)
	return nil, false
}

// ---------------------------------------------------------------------------
// Free index discovery
// ---------------------------------------------------------------------------

// findFreeRFCOMMIndex returns the lowest RFCOMM index (0..31) not present in
// the existing map.  Returns -1 if all indices are occupied (very unlikely).
func findFreeRFCOMMIndex(existing map[int]string) int {
	for i := 0; i <= 31; i++ {
		if _, used := existing[i]; !used {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// Bind RFCOMM device
// ---------------------------------------------------------------------------

// bindRFCOMM executes `rfcomm bind <index> <MAC> <channel>` and waits for
// the /dev/rfcommX device node to appear (up to 2 s).
// Requires CAP_NET_ADMIN or root.
func bindRFCOMM(mac string, channel int) (*rfcommBinding, error) {
	mac = NormalizeMAC(mac)

	// Snapshot existing bindings so we can find a free slot.
	existing, err := listRFCOMMBindings()
	if err != nil {
		// rfcomm binary likely missing; abort.
		return nil, fmt.Errorf("BT/RFCOMM: cannot list bindings (is rfcomm installed?): %w", err)
	}

	// Check once more: maybe the MAC got bound between our last check and now.
	for idx, boundMAC := range existing {
		if boundMAC == mac {
			devPath := fmt.Sprintf("/dev/rfcomm%d", idx)
			logger.Infof("BT/RFCOMM: bind skipped — %s already bound as %s", mac, devPath)
			return &rfcommBinding{DevPath: devPath, Index: idx, Channel: channel}, nil
		}
	}

	idx := findFreeRFCOMMIndex(existing)
	if idx < 0 {
		return nil, fmt.Errorf("BT/RFCOMM: no free RFCOMM index available (all 0..31 are occupied)")
	}

	devPath := fmt.Sprintf("/dev/rfcomm%d", idx)
	args := []string{"bind", strconv.Itoa(idx), mac, strconv.Itoa(channel)}

	logger.Infof("BT/RFCOMM: running: rfcomm %s", strings.Join(args, " "))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rfcomm", args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("BT/RFCOMM: rfcomm bind timed out")
	}
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		// Provide an actionable hint for permission errors.
		if strings.Contains(outStr, "Operation not permitted") ||
			strings.Contains(outStr, "permission denied") ||
			strings.Contains(err.Error(), "permission denied") {
			return nil, fmt.Errorf(
				"BT/RFCOMM: rfcomm bind failed — insufficient privileges "+
					"(run as root or add user to 'bluetooth'/'dialout' group): %w; output: %s",
				err, outStr,
			)
		}
		return nil, fmt.Errorf("BT/RFCOMM: rfcomm bind failed: %w; output: %s", err, outStr)
	}

	logger.Infof("BT/RFCOMM: rfcomm bind succeeded, waiting for %s to appear…", devPath)

	// Wait up to 2 s for the device node to be created by the kernel.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(devPath); err == nil {
			logger.Infof("BT/RFCOMM: device %s is ready", devPath)
			return &rfcommBinding{DevPath: devPath, Index: idx, Channel: channel}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("BT/RFCOMM: %s did not appear after rfcomm bind", devPath)
}

// ---------------------------------------------------------------------------
// serialConn — net.Conn wrapper around *os.File
// ---------------------------------------------------------------------------

// serialConn implements net.Conn over an *os.File (a /dev/rfcommX device).
// It does not support Read timeouts out of the box; only write deadlines are
// practically needed for printing.
type serialConn struct {
	f    *os.File
	path string
}

type serialAddr struct{ path string }

func (a serialAddr) Network() string { return "rfcomm-serial" }
func (a serialAddr) String() string  { return a.path }

func (c *serialConn) Read(b []byte) (int, error)  { return c.f.Read(b) }
func (c *serialConn) Write(b []byte) (int, error) { return c.f.Write(b) }
func (c *serialConn) Close() error                { return c.f.Close() }

func (c *serialConn) LocalAddr() net.Addr  { return serialAddr{c.path} }
func (c *serialConn) RemoteAddr() net.Addr { return serialAddr{c.path} }

func (c *serialConn) SetDeadline(t time.Time) error {
	return c.f.SetDeadline(t)
}
func (c *serialConn) SetReadDeadline(t time.Time) error {
	return c.f.SetReadDeadline(t)
}
func (c *serialConn) SetWriteDeadline(t time.Time) error {
	return c.f.SetWriteDeadline(t)
}

// ---------------------------------------------------------------------------
// Open an RFCOMM device
// ---------------------------------------------------------------------------

// openRFCOMMDevice opens /dev/rfcommX for read/write and returns a net.Conn.
// Does not require elevated privileges when the device already exists and the
// user is a member of the 'dialout' or 'bluetooth' group.
func openRFCOMMDevice(b *rfcommBinding) (net.Conn, error) {
	f, err := os.OpenFile(b.DevPath, os.O_RDWR, 0)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf(
				"BT/RFCOMM: cannot open %s — add user to 'dialout' or 'bluetooth' group: %w",
				b.DevPath, err,
			)
		}
		return nil, fmt.Errorf("BT/RFCOMM: failed to open %s: %w", b.DevPath, err)
	}
	logger.Infof("BT/RFCOMM: opened %s as serial connection", b.DevPath)
	return &serialConn{f: f, path: b.DevPath}, nil
}

// ---------------------------------------------------------------------------
// Full orchestration
// ---------------------------------------------------------------------------

// ensureRFCOMMDevice finds or creates a /dev/rfcommX device for mac and
// returns its binding.  sdpChannel is the channel from SDP discovery (0 if
// SDP failed).  All steps are logged.  The result is cached for future calls.
func ensureRFCOMMDevice(mac string, sdpChannel int) (*rfcommBinding, error) {
	mac = NormalizeMAC(mac)

	// 1. Cache hit → fast path.
	if b, ok := globalRFCOMMCache.get(mac); ok {
		logger.Debugf("BT/RFCOMM: cache hit for %s → %s (channel %d)", mac, b.DevPath, b.Channel)
		// Verify the device node still exists (e.g. after a reboot the node may be gone).
		if _, err := os.Stat(b.DevPath); err == nil {
			return b, nil
		}
		logger.Warnf("BT/RFCOMM: cached device %s no longer exists; re-binding", b.DevPath)
	}

	// 2. Check for an existing kernel binding for this MAC.
	if existing, ok := findExistingRFCOMMDevice(mac); ok {
		// Reuse it; update channel from SDP if we have one.
		if sdpChannel > 0 {
			existing.Channel = sdpChannel
		}
		// If channel is still 0 after SDP and existing, default to 1.
		if existing.Channel == 0 {
			existing.Channel = 1
		}
		logger.Infof("BT/RFCOMM: reusing existing device %s for %s (channel %d)",
			existing.DevPath, mac, existing.Channel)
		globalRFCOMMCache.set(mac, existing)
		return existing, nil
	}

	// 3. No existing binding — we need to create one.
	// Determine the channel to bind on.
	channel := sdpChannel

	if channel > 0 {
		logger.Infof("BT/RFCOMM: using SDP-discovered channel %d for %s", channel, mac)
	} else {
		logger.Infof("BT/RFCOMM: SDP gave no channel for %s; will probe channels 1..8 after bind", mac)
		// Default to channel 1; probe happens in dialRFCOMMLinux if the connection fails.
		channel = 1
	}

	b, err := bindRFCOMM(mac, channel)
	if err != nil {
		return nil, err
	}

	globalRFCOMMCache.set(mac, b)
	logger.Infof("BT/RFCOMM: bound %s → %s (channel %d)", mac, b.DevPath, b.Channel)
	return b, nil
}

// ---------------------------------------------------------------------------
// Linux entry point for Bluetooth dialling
// ---------------------------------------------------------------------------

// dialRFCOMMLinux is the Linux-specific entry point for establishing a
// Bluetooth RFCOMM connection.  Strategy:
//
//  1. Run SDP discovery to find the channel.
//  2. Call ensureRFCOMMDevice to get (or create) /dev/rfcommX.
//  3. Open the device as a serial net.Conn.
//  4. If SDP failed, probe channels 1–8 by re-binding and retrying.
//  5. If the device approach fails entirely, fall back to the raw RFCOMM socket.
func dialRFCOMMLinux(mac string, cachedChannel int) (net.Conn, error) {
	mac = NormalizeMAC(mac)

	// ── Step 1: SDP discovery ────────────────────────────────────────────────
	var sdpChannel int
	if cachedChannel > 0 {
		sdpChannel = cachedChannel
		logger.Debugf("BT/RFCOMM: using cached channel %d for %s", cachedChannel, mac)
	} else {
		ch, err := sdpDiscoverLinux(mac)
		if err != nil {
			logger.Warnf("BT/RFCOMM: SDP discovery failed for %s: %v", mac, err)
			sdpChannel = 0
		} else {
			sdpChannel = ch
			logger.Infof("BT/RFCOMM: SDP discovered channel %d for %s", ch, mac)
		}
	}

	// ── Step 2 & 3: Get/create device and open it ───────────────────────────
	if sdpChannel > 0 {
		conn, err := tryRFCOMMDevice(mac, sdpChannel)
		if err == nil {
			return conn, nil
		}
		logger.Warnf("BT/RFCOMM: device connection on SDP channel %d failed for %s: %v; probing", sdpChannel, mac, err)
	}

	// ── Step 4: Channel probing (SDP failed or device connection failed) ─────
	// Common thermal printer channels.
	probeChannels := []int{1, 2, 3, 4, 5, 6, 7, 8}
	// Don't re-probe the channel we already tried.
	if sdpChannel > 0 {
		filtered := probeChannels[:0]
		for _, ch := range probeChannels {
			if ch != sdpChannel {
				filtered = append(filtered, ch)
			}
		}
		probeChannels = filtered
	}

	for _, ch := range probeChannels {
		logger.Infof("BT/RFCOMM: probing channel %d for %s", ch, mac)
		conn, err := tryRFCOMMDevice(mac, ch)
		if err == nil {
			logger.Infof("BT/RFCOMM: channel probe succeeded on channel %d for %s", ch, mac)
			return conn, nil
		}
		logger.Debugf("BT/RFCOMM: channel %d probe failed for %s: %v", ch, mac, err)
		// Remove the failed cache entry so the next iteration can try a fresh bind.
		globalRFCOMMCache.mu.Lock()
		delete(globalRFCOMMCache.entries, mac)
		globalRFCOMMCache.mu.Unlock()
	}

	// ── Step 5: Raw socket fallback ──────────────────────────────────────────
	logger.Warnf("BT/RFCOMM: /dev/rfcommX approach exhausted for %s; falling back to raw RFCOMM socket", mac)

	fallbackCh := sdpChannel
	if fallbackCh == 0 {
		fallbackCh = 1
	}
	conn, err := dialRFCOMM(mac, fallbackCh)
	if err != nil {
		return nil, fmt.Errorf("BT/RFCOMM: all connection strategies failed for %s: %w", mac, err)
	}
	logger.Infof("BT/RFCOMM: raw socket fallback succeeded for %s on channel %d", mac, fallbackCh)
	return conn, nil
}

// tryRFCOMMDevice is a helper that calls ensureRFCOMMDevice with the given
// channel and then opens the resulting device.  It invalidates the cache entry
// on failure so the caller can retry with a different channel.
func tryRFCOMMDevice(mac string, channel int) (net.Conn, error) {
	// Force cache invalidation for the given channel attempt so ensureRFCOMMDevice
	// will create a fresh binding with the requested channel.
	// (Only invalidate if the cached entry has a *different* channel.)
	if b, ok := globalRFCOMMCache.get(mac); ok && b.Channel != channel {
		globalRFCOMMCache.mu.Lock()
		delete(globalRFCOMMCache.entries, mac)
		globalRFCOMMCache.mu.Unlock()
	}

	b, err := ensureRFCOMMDevice(mac, channel)
	if err != nil {
		return nil, err
	}

	conn, err := openRFCOMMDevice(b)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
