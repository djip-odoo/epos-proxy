package printer

import "epos-proxy/internal/config"

type BTPrinterInfo struct {
	Id      string
	Address string
	Name    string
}

func ListBluetoothPrinters(cfg *config.Manager) []BTPrinterInfo {
	printers := make([]BTPrinterInfo, 0)

	btPrinters := cfg.GetBluetoothPrinters()
	for _, btCfg := range btPrinters {
		if btCfg.Name == "" {
			btCfg.Name = "BT - " + btCfg.Address
		}
		printers = append(printers, BTPrinterInfo{
			Id:      EncodeBluetoothPrinterID(btCfg.Address),
			Address: btCfg.Address,
			Name:    btCfg.Name,
		})
	}
	return printers
}
