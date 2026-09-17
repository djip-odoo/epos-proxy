import { main } from "../../wailsjs/go/models";
import { PrinterContext } from "../contexts/PrinterContext";
import { useContext } from "react";
import PrinterActions from "./PrinterActions";
import LibusbFixDialog from "./LibusbFixDialog";
import CloseButton from "./CloseButton";
import { PrinterTypeIcon } from "../functions/printerIcons";

type PrinterListItemProps =
  | {
    printer: main.Printer;
    isOnline: true;
  }
  | {
    printer: main.UnavailablePrinter;
    isOnline: false;
  };

export default function PrinterListItem(props: PrinterListItemProps) {
  const { data, actions } = useContext(PrinterContext);

  if (props.isOnline) {
    const { printer } = props;
    const status = printer.isBT && printer.btMac
      ? data.btStatus[printer.btMac]
      : printer.isLAN && printer.lanIp
        ? data.lanStatus[printer.lanIp]
        : null;

    const statusColor =
      status === "online"
        ? "text-success"
        : status === "offline"
          ? "text-danger"
          : status === "loading"
            ? "text-warning"
            : printer.online
              ? "text-success"
              : "text-danger";

    const handleRemove = printer.isLAN
      ? () => actions.removeLanPrinter(printer)
      : printer.isBT
        ? () => actions.removeBluetoothPrinter(printer)
        : null;

    return (
      <li key={printer.id} className="text-left first:pt-0 py-6 last:pb-0 relative">
        <div className="flex items-center justify-between gap-2.5">
          <span className={`shrink-0 ${statusColor}`}>
            <PrinterTypeIcon printer={printer} className="w-5 h-5" />
          </span>
          <span className="min-w-0 font-medium text-gray-900 break-all flex-1">
            {printer.name}
          </span>
          {handleRemove && <CloseButton onClick={handleRemove} />}
        </div>
        <div className="text-gray-600 mt-2 text-sm break-all">{printer.ip}</div>
        <PrinterActions printer={printer} />
      </li>
    );
  }

  const { printer } = props;
  return (
    <li key={printer.name} className="text-left first:pt-0 py-6 last:pb-0 relative">
      <div className="flex items-center gap-2.5">
        <span className="shrink-0 text-danger">
          <PrinterTypeIcon printer={printer} className="w-5 h-5" />
        </span>
        <span className="min-w-0 font-medium text-gray-900">{printer.name}</span>
      </div>
      <div className="text-danger mt-1 text-wrap">
        Unable to communicate with this printer: {printer.errorMsg}
      </div>
      {printer.errorMsg?.toLowerCase().includes("libusb") && (
        <LibusbFixDialog printerName={printer.name} />
      )}
    </li>
  );
}
