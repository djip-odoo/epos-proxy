package bluetooth

// rfcommBinding records the state of a bound (or candidate) RFCOMM device.
// On Linux the DevPath refers to an actual /dev/rfcommX node;
// on Darwin/Windows it is used only as a value holder.
type rfcommBinding struct {
	DevPath string // e.g. "/dev/rfcomm0"
	Channel int    // RFCOMM channel number
	Index   int    // numeric index (0 = rfcomm0, 1 = rfcomm1, …)
}

func (bm *BluetoothManager) setBinding(address string, b *rfcommBinding) {
	if b == nil {
		return
	}
	address = normalizeAddress(address)
	bm.cache.set(address, b)
}

func (bm *BluetoothManager) deleteBinding(address string) {
	address = normalizeAddress(address)
	bm.cache.deleteIf(address, nil)
}

func (bm *BluetoothManager) deleteBindingIfMatching(address string, expected *rfcommBinding) {
	if expected == nil {
		return
	}
	address = normalizeAddress(address)
	bm.cache.deleteIf(address, func(b *rfcommBinding) bool {
		return b != nil && b.Channel == expected.Channel && b.DevPath == expected.DevPath && b.Index == expected.Index
	})
}

func (bm *BluetoothManager) deleteBindingForChannel(address string, channel int) {
	address = normalizeAddress(address)
	bm.cache.deleteIf(address, func(b *rfcommBinding) bool {
		return b != nil && (channel <= 0 || b.Channel == channel)
	})
}

// netAddrPlaceholder implements net.Addr for Bluetooth network connections across platforms (RFCOMM, BLE).
type netAddrPlaceholder struct {
	net  string
	addr string
}

func (a netAddrPlaceholder) Network() string { return a.net }
func (a netAddrPlaceholder) String() string  { return a.addr }
