package printer

import (
	"errors"
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

type mockLANDriver struct {
	mockDriver
	ips      []string
	checkErr error
}

func (m *mockLANDriver) Add(ip string) (Printer, error) {
	return &mockPrinter{id: ip}, nil
}

func (m *mockLANDriver) Remove(ip string) (string, error) {
	return ip, nil
}

func (m *mockLANDriver) GetConfiguredIPs() []string {
	return m.ips
}

func (m *mockLANDriver) CheckLANPrinter(ip string) error {
	return m.checkErr
}

var _ LANDriver = (*mockLANDriver)(nil)

func TestDriverRegistry_NewDefaultManager(t *testing.T) {
	defer ResetDriversForTesting()
	ResetDriversForTesting()

	RegisterDriver("mock1", func(cfg *config.Manager) Driver {
		return &mockDriver{name: "driver1"}
	})
	RegisterDriver("mock2", func(cfg *config.Manager) Driver {
		return &mockDriver{name: "driver2"}
	})

	// Overwrite mock1
	RegisterDriver("mock1", func(cfg *config.Manager) Driver {
		return &mockDriver{name: "driver1_updated"}
	})

	mgr := RegistryManager(nil)
	defer mgr.Close()

	testutil.ExpectedNotNil(t, mgr)
	testutil.ExpectedLen(t, mgr.drivers, 2)
	testutil.ExpectedEqual(t, mgr.drivers[0].Name(), "driver1_updated")
	testutil.ExpectedEqual(t, mgr.drivers[1].Name(), "driver2")
}

func TestManager_CheckLANPrinterStatus(t *testing.T) {
	mgr := NewManager()

	// When no LAN driver is registered
	testutil.ExpectedFalse(t, mgr.CheckLANPrinterStatus("192.168.1.50"))

	// When LAN driver is registered and returns error
	lanDrv := &mockLANDriver{
		mockDriver: mockDriver{name: "mockLAN"},
		checkErr:   errors.New("host unreachable"),
	}
	mgr.RegisterDriver(lanDrv)
	testutil.ExpectedFalse(t, mgr.CheckLANPrinterStatus("192.168.1.50"))

	// When LAN driver returns nil (success)
	lanDrv.checkErr = nil
	testutil.ExpectedTrue(t, mgr.CheckLANPrinterStatus("192.168.1.50"))
}
