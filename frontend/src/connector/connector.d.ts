export interface Printer {
    name: string;
    ip: string;
    id: string;
    isLAN: boolean;
    lanIp?: string;
    online: boolean;
}

export interface UnavailablePrinter {
    name: string;
    errorMsg: string;
    isLAN: boolean;
    lanIp?: string;
}

export interface StatusResponse {
    serverRunning: boolean;
    defaultIp: string;
    errorMsg: string;
    printers: Printer[];
    unavailablePrinters: UnavailablePrinter[];
    os: string;
}

export interface LANSettings {
    enabled: boolean;
    ip: string;
    port: number;
}

export interface PrinterSettingConfig {
    id: string;
    width: number;
    bottom_padding: number;
    protocol: string;
    vid_pid?: string;
    device_id?: Record<string, string>;
    cash_drawer_pin: number;
}

export interface ConnectorError {
    code: string;
    message: string;
}

export interface CustomerDisplayURL {
    id: string;
    name: string;
    url: string;
    description: string;
    enabled: boolean;
    createdAt: string;
    updatedAt: string;
}

export interface MonitorInfo {
    id: string;
    name: string;
    width: number;
    height: number;
    x: number;
    y: number;
    isPrimary: boolean;
}

export interface Connector {
    status(): Promise<StatusResponse>;
    addLANPrinter(ip: string): Promise<void>;
    confirmRemoveLANPrinter(ip: string): Promise<boolean>;
    checkLANPrinterStatus(ip: string): Promise<boolean>;
    getLANSettings(): Promise<LANSettings>;
    enableLANAccess(): Promise<void>;
    disableLANAccess(): Promise<void>;
    isAutostartEnabled(): Promise<boolean>;
    enableAutostart(): Promise<void>;
    disableAutostart(): Promise<void>;
    getPrinterSetting(id: string): Promise<PrinterSettingConfig>;
    setPrinterSetting(id: string, width: number, bottomPadding: number, protocol: string, cashDrawerPin: number): Promise<void>;
    downloadLogs(): Promise<void>;
    browserOpenURL(url: string): Promise<void>;
    setLANPin(pin: string): Promise<void>;
    verifyPin(pin: string): Promise<{ token: string }>;
    // Customer Display WebView
    getCustomerDisplayURLs(): Promise<CustomerDisplayURL[]>;
    getActiveCustomerDisplayURL(): Promise<CustomerDisplayURL | null>;
    addCustomerDisplayURL(name: string, url: string, description: string): Promise<CustomerDisplayURL>;
    updateCustomerDisplayURL(id: string, name: string, url: string, description: string, enabled: boolean): Promise<void>;
    setActiveCustomerDisplayURL(id: string): Promise<void>;
    disableCustomerDisplayURL(id: string): Promise<void>;
    validateAdminPin(pin: string): Promise<boolean>;
    setWindowFullscreen(fullscreen: boolean): Promise<void>;
    isCustomerDisplayOpen(): Promise<boolean>;
    setCustomerDisplayOpen(open: boolean): Promise<void>;
    getMonitors(): Promise<MonitorInfo[]>;
    saveMonitorSelection(monitorID: string, remember: boolean): Promise<void>;
    getMonitorSelection(): Promise<[string, boolean]>;
    identifyDisplays(): Promise<void>;
    testCustomerDisplay(monitorID: string): Promise<void>;
    openCustomerDisplayWindow(monitorID: string, url: string): Promise<void>;
    closeCustomerDisplayWindow(): Promise<void>;
    reloadCustomerDisplayWindow(): Promise<void>;
    navigateCustomerDisplayWindow(url: string): Promise<void>;
}
