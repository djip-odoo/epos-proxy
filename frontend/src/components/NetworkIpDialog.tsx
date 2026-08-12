import { useContext, useState } from "react";
import { PrinterContext } from "../contexts/PrinterContext";
import Dialog from "./Dialog";

export default function NetworkIpDialog() {
  const printerContext = useContext(PrinterContext);
  const [ipInput, setIpInput] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const submit = async () => {
    const ip = ipInput.trim();
    if (!ip) {
      setErrorMessage("IP address cannot be empty.");
      return false;
    }

    const result = await printerContext.actions.addLanPrinter(ip);
    if (!result.status) {
      setErrorMessage(result.message);
      return false;
    }

    return true;
  };

  const cleanup = () => {
    setIpInput("");
    setErrorMessage(null);
  };

  return (
    <Dialog
      title="Add Network Printer"
      validateText="Submit"
      validateCallback={submit}
      isValidateDisabled={ipInput.trim() === ""}
      onClose={cleanup}
      openButton={
        <div className="border-2 border-dashed border-gray-300 bg-gray-50 rounded-lg px-4 py-3 text-gray-600 hover:border-gray-400 hover:bg-gray-100 cursor-pointer">
          + Add Network Printer
        </div>
      }
    >
      <input
        autoFocus
        type="text"
        placeholder="IP Address (e.g. 192.168.1.100)"
        className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-odoo-light focus:border-transparent mb-3"
        value={ipInput}
        onChange={(event) => setIpInput(event.target.value)}
      />
      {errorMessage && (
        <div
          className="bg-red-100 border border-red-400 text-red-700 rounded-md text-sm px-4 mb-3 py-3 relative"
          role="alert"
        >
          {errorMessage}
        </div>
      )}
    </Dialog>
  );
}
