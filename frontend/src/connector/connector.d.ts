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
}
