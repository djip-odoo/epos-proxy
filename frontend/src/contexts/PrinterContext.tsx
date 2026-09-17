import { main } from "../../wailsjs/go/models";
import {
  AddLANPrinter,
  CheckLANPrinterStatus,
  ConfirmRemoveLANPrinter,
  AddBluetoothPrinter,
  CheckBluetoothPrinterStatus,
  ConfirmRemoveBluetoothPrinter,
  IsNetworkPrintingEnabled,
  Printers,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { createContext, useCallback, useEffect, useRef, useState, type ReactNode } from "react";

const POLL_INTERVAL = 5000;
const FETCH_ERROR = "Failed to retrieve printer status. Please try again.";

export type PrinterStatus = "loading" | "online" | "offline";
export type PrinterStatusMap = Record<string, PrinterStatus>;

export type ActionStatus = {
  status: boolean;
  message: string;
};

export type PrinterContextType = {
  setters: {};
  data: {
    printers: main.Printers | null;
    lanStatus: PrinterStatusMap;
    btStatus: PrinterStatusMap;
    fetchError: string | null;
    networkPrintingEnabled: boolean;
  };
  actions: {
    removeLanPrinter: (printer: main.Printer) => Promise<ActionStatus>;
    addLanPrinter: (ip: string) => Promise<ActionStatus>;
    removeBluetoothPrinter: (printer: main.Printer) => Promise<ActionStatus>;
    addBluetoothPrinter: (address: string, name: string) => Promise<ActionStatus>;
    checkBtPrinterStatus: (mac: string) => Promise<void>;
    checkLanPrinterStatus: (ip: string) => Promise<void>;
    refreshPrinters: (force?: boolean) => Promise<void>;
  };
};

export const PrinterContext = createContext({} as PrinterContextType);

export const PrinterContextWrapper = ({ children }: { children: ReactNode }) => {
  const [printers, setPrinters] = useState<main.Printers | null>(null);
  const [lanStatus, setLanStatus] = useState<PrinterStatusMap>({});
  const [btStatus, setBtStatus] = useState<PrinterStatusMap>({});
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [networkPrintingEnabled, setNetworkPrintingEnabledState] = useState(false);

  const statusChecksInFlight = useRef(0);
  const pendingLanChecks = useRef<Set<string>>(new Set());
  const pendingBtChecks = useRef<Set<string>>(new Set());

  // Generic status checker for LAN and Bluetooth
  const checkStatus = useCallback(
    async (
      id: string,
      pendingSet: React.MutableRefObject<Set<string>>,
      setMap: React.Dispatch<React.SetStateAction<PrinterStatusMap>>,
      checker: (target: string) => Promise<boolean>,
      typeLabel: string,
    ) => {
      if (pendingSet.current.has(id)) return;
      pendingSet.current.add(id);

      setMap((prev) => (prev[id] === undefined ? { ...prev, [id]: "loading" } : prev));

      try {
        const isOnline = await checker(id);
        setMap((prev) => ({ ...prev, [id]: isOnline ? "online" : "offline" }));
      } catch (error) {
        console.error(`Failed to check ${typeLabel} printer status for ${id}:`, error);
      } finally {
        pendingSet.current.delete(id);
      }
    },
    [],
  );

  const checkLanPrinterStatus = useCallback(
    (ip: string) => checkStatus(ip, pendingLanChecks, setLanStatus, CheckLANPrinterStatus, "LAN"),
    [checkStatus],
  );

  const checkBtPrinterStatus = useCallback(
    (mac: string) => checkStatus(mac, pendingBtChecks, setBtStatus, CheckBluetoothPrinterStatus, "Bluetooth"),
    [checkStatus],
  );

  const checkAppStatus = useCallback(
    async (force = false) => {
      if (statusChecksInFlight.current > 0 && !force) return;

      statusChecksInFlight.current++;
      try {
        const data = await Printers();
        setPrinters(data);
        setFetchError(null);

        for (const printer of data.printers) {
          if (printer.isLAN && printer.lanIp) checkLanPrinterStatus(printer.lanIp);
          if (printer.isBT && printer.btMac) checkBtPrinterStatus(printer.btMac);
        }
      } catch (error) {
        console.error("Failed to check app status:", error);
        setFetchError(FETCH_ERROR);
      } finally {
        statusChecksInFlight.current--;
      }
    },
    [checkLanPrinterStatus, checkBtPrinterStatus],
  );

  const removeLanPrinter = async (printer: main.Printer): Promise<ActionStatus> => {
    if (!printer.isLAN || !printer.lanIp) {
      return { status: false, message: "Cannot remove a non-LAN printer" };
    }
    try {
      const confirmed = await ConfirmRemoveLANPrinter(printer.lanIp);
      if (!confirmed) throw new Error("User cancelled removal");
      await checkAppStatus(true);
      return { status: true, message: `Successfully removed LAN printer with IP ${printer.lanIp}` };
    } catch (error) {
      return { status: false, message: `Failed to remove LAN printer: ${error}` };
    }
  };

  const addLanPrinter = async (ip: string): Promise<ActionStatus> => {
    try {
      await AddLANPrinter(ip);
      await checkAppStatus(true);
      return { status: true, message: `Successfully added LAN printer with IP ${ip}` };
    } catch (error) {
      return { status: false, message: `Failed to add LAN printer: ${error}` };
    }
  };

  const removeBluetoothPrinter = async (printer: main.Printer): Promise<ActionStatus> => {
    if (!printer.isBT || !printer.btMac) {
      return { status: false, message: "Cannot remove a non-Bluetooth printer" };
    }
    try {
      const confirmed = await ConfirmRemoveBluetoothPrinter(printer.btMac);
      if (!confirmed) throw new Error("User cancelled removal");
      await checkAppStatus(true);
      return { status: true, message: `Successfully removed Bluetooth printer ${printer.name || printer.btMac}` };
    } catch (error) {
      return { status: false, message: `Failed to remove Bluetooth printer: ${error}` };
    }
  };

  const addBluetoothPrinter = async (address: string, name: string): Promise<ActionStatus> => {
    try {
      await AddBluetoothPrinter(address, name);
      await checkAppStatus(true);
      return { status: true, message: `Successfully added Bluetooth printer ${name || address}` };
    } catch (error) {
      return { status: false, message: `Failed to add Bluetooth printer: ${error}` };
    }
  };

  // Poll only while window is visible and focused
  useEffect(() => {
    let intervalId: number | null = null;

    const startPolling = () => {
      if (intervalId !== null) return;
      checkAppStatus();
      intervalId = window.setInterval(checkAppStatus, POLL_INTERVAL);
    };

    const stopPolling = () => {
      if (intervalId === null) return;
      clearInterval(intervalId);
      intervalId = null;
    };

    const handleVisibilityChange = () => (document.hidden ? stopPolling() : startPolling());

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("focus", startPolling);
    window.addEventListener("blur", stopPolling);

    if (!document.hidden) startPolling();

    return () => {
      stopPolling();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("focus", startPolling);
      window.removeEventListener("blur", stopPolling);
    };
  }, [checkAppStatus]);

  const loadNetworkPrintingStatus = useCallback(async () => {
    try {
      const enabled = await IsNetworkPrintingEnabled();
      setNetworkPrintingEnabledState(enabled);
    } catch (err) {
      console.error("Failed to load network printing status", err);
    }
  }, []);

  useEffect(() => {
    loadNetworkPrintingStatus();
  }, [loadNetworkPrintingStatus]);

  useEffect(() => {
    return EventsOn("network-printing-changed", () => {
      loadNetworkPrintingStatus();
      checkAppStatus(true);
    });
  }, [loadNetworkPrintingStatus, checkAppStatus]);

  return (
    <PrinterContext.Provider
      value={{
        data: { printers, lanStatus, btStatus, fetchError, networkPrintingEnabled },
        setters: {},
        actions: {
          removeLanPrinter,
          addLanPrinter,
          removeBluetoothPrinter,
          addBluetoothPrinter,
          checkBtPrinterStatus,
          checkLanPrinterStatus,
          refreshPrinters: checkAppStatus,
        },
      }}
    >
      {children}
    </PrinterContext.Provider>
  );
};
