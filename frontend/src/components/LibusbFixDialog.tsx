import { useContext, useMemo } from "react";
import { AppContext } from "../contexts/AppContext";
import { brewSteps, linuxSteps, zadigSteps } from "../assets/data/fixStep";
import type { Step } from "../types";
import StepDialog from "./StepDialog";

export default function LibusbFixDialog({ printerName }: { printerName: string; }) {
  const appContext = useContext(AppContext);
  const { isWindows, isMac, isLinux } = appContext.data;

  const fixSteps = useMemo<Step[]>(() => {
    if (isWindows) {
      return zadigSteps(printerName);
    }
    if (isMac) {
      return brewSteps(printerName);
    }
    if (isLinux) {
      return linuxSteps(printerName);
    }
    return [];
  }, [printerName, isWindows, isMac, isLinux]);

  if (fixSteps.length === 0) {
    return null;
  }
  const title = "Install Printer Driver";
  return (
    <StepDialog
      steps={fixSteps}
      title={title}
      openButton={
        <div className="mt-6 text-center">
          <div className="flex-1 border bg-odoo text-white hover:bg-odoo-dark rounded-lg px-4 py-2 text-center cursor-pointer mt-2 text-sm font-medium">
            {`Fix - ${title}`}
          </div>
        </div>
      }
    />
  );
}
