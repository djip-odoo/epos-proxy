//go:build darwin && cgo

package bluetooth

/*
#include <stdint.h>
#include <objc/runtime.h>
#include <objc/message.h>

// Query CBCharacteristic.properties via the ObjC runtime.
// CBCharacteristicProperties is NSUInteger (= uint64_t on arm64/amd64).
static uint64_t cbchr_properties(void* p) {
	typedef uint64_t (*msgFn)(id, SEL);
	return ((msgFn)objc_msgSend)((id)p, sel_registerName("properties"));
}
*/
import "C"
import (
	"epos-proxy/logger"
	"fmt"
	"unsafe"

	tinygoBT "tinygo.org/x/bluetooth"
)

// CBCharacteristicProperties write-capable bits (stable Apple API values).
const (
	cbPropWriteWithoutResponse uint64 = 0x04
	cbPropWrite                uint64 = 0x08
)

// Overrides the cross-platform fallback in
// ble_charprop_other.go. On Darwin+CGO we can read CBCharacteristicProperties
// via the ObjC runtime and prefer a characteristic that has Write or
// WriteWithoutResponse.
func discoverPrinterCharacteristic(service tinygoBT.DeviceService) (*tinygoBT.DeviceCharacteristic, error) {
	chars, err := service.DiscoverCharacteristics(nil)
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 {
		return nil, fmt.Errorf("BT/ble: service %s exposes no characteristics", service.UUID())
	}

	var fallback *tinygoBT.DeviceCharacteristic
	for i := range chars {
		c := &chars[i]
		props := cbCharProperties(c)
		logger.Debugf("BT/ble: characteristic %s props=0x%x", c.UUID(), props)
		if props&(cbPropWrite|cbPropWriteWithoutResponse) != 0 {
			logger.Debugf("BT/ble: selected writable characteristic %s", c.UUID())
			return c, nil
		}
		if fallback == nil {
			fallback = c
		}
	}

	logger.Debugf("BT/ble: no writable characteristic found, using fallback %s", fallback.UUID())
	return fallback, nil
}

// Extracts CBCharacteristicProperties from a DeviceCharacteristic
// using unsafe arithmetic on the tinygo.org/x/bluetooth v0.15.0 struct layout:
//
//	DeviceCharacteristic { *deviceCharacteristic }        — one pointer (8 B)
//	deviceCharacteristic {
//	    uuidWrapper      [16]byte   offset  0
//	    service          DeviceService (= {*deviceService}) offset 16
//	    characteristic   cbgo.Characteristic{ptr unsafe.Pointer} offset 24  ← target
//	    ...
//	}
//
// cbgo.Characteristic.ptr holds the raw ObjC CBCharacteristic* object.
func cbCharProperties(c *tinygoBT.DeviceCharacteristic) uint64 {
	// Step 1: read the embedded *deviceCharacteristic pointer.
	dcPtr := *(*unsafe.Pointer)(unsafe.Pointer(c))
	// Step 2: at offset 24 sits cbgo.Characteristic{ptr unsafe.Pointer};
	//         the ptr field is at offset 0 within that struct.
	chrPtr := *(*unsafe.Pointer)(unsafe.Pointer(uintptr(dcPtr) + 24))
	// Step 3: query the ObjC object for its properties bitmask.
	return uint64(C.cbchr_properties(chrPtr))
}

// Resolves a Classic MAC address to a BLE UUID on macOS by
// querying system_profiler for the device's Bluetooth name, then scanning for a
// BLE device with a matching or similar name.
func resolveMACToBLEUUID(mac string) (string, bool) {
	btName := lookupBluetoothName(mac)
	if btName == "" {
		return "", false
	}
	sanitizedTarget := sanitizeForCUName(btName)
	if sanitizedTarget == "" {
		return "", false
	}

	logger.Infof("BT/darwin/classic: attempting to resolve MAC %s (%q) via BLE scan name-matching", mac, btName)

	ble := &BLETransport{}
	if !ble.IsAvailable() {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	devices, err := ble.Scan(ctx)
	if err != nil {
		return "", false
	}

	for _, dev := range devices {
		if strings.Contains(strings.ToLower(sanitizeForCUName(dev.Name)), sanitizedTarget) {
			logger.Infof("BT/darwin/classic: matched BLE device %s (%q) for classic printer %s", dev.MAC, dev.Name, mac)
			return dev.MAC, true
		}
	}

	return "", false
}

func lookupBluetoothName(mac string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "system_profiler", "SPBluetoothDataType", "-json").Output()
	if ctx.Err() == context.DeadlineExceeded {
		logger.Warnf("BT/darwin: system_profiler timed out resolving BT name for %s", mac)
		return ""
	}
	if err != nil {
		logger.Warnf("BT/darwin: system_profiler failed, cannot resolve BT name for %s: %v", mac, err)
		return ""
	}

	var generic map[string]any
	if err := json.Unmarshal(out, &generic); err != nil {
		logger.Warnf("BT/darwin: failed to parse system_profiler JSON: %v", err)
		return ""
	}

	target := strings.ToLower(mac)
	var found string
	var walk func(v any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch t := v.(type) {
		case map[string]any:
			if addr, ok := t["device_address"].(string); ok && strings.ToLower(addr) == target {
				if name, ok := t["_name"].(string); ok {
					found = name
					return
				}
			}
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(generic)

	if found == "" {
		logger.Debugf("BT/darwin: no system_profiler entry matched MAC %s", mac)
	} else {
		logger.Infof("BT/darwin: resolved MAC %s -> Bluetooth name %q", mac, found)
	}
	return found
}

func sanitizeForCUName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}
