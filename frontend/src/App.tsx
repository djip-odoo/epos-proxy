import { useEffect, useState } from "react";
import NetworkPrinting from "./components/NetworkPrinting";
import NetworkPrintingEnabledDialog from "./components/NetworkPrintingEnabledDialog";
import PrinterList from "./components/PrinterList";
import SetPinDialog from "./components/SetPinDialog";
import PINModal from "./components/PINModal";
import { AppContextWrapper } from "./contexts/AppContext";
import { PINContextWrapper } from "./contexts/PINContext";
import { PrinterContextWrapper } from "./contexts/PrinterContext";
import { ToastContextWrapper } from "./contexts/ToastContext";
import { WebViewContextWrapper } from "./contexts/WebViewContext";
import { backendService } from "./services/backend";

function AppContent() {
  const [pendingPin, setPendingPin] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      backendService.setWailsAppURL(
        window.location.origin + window.location.pathname
      );
    }

    const isExit =
      typeof window !== "undefined" &&
      (window.location.search.includes("kiosk_exit=1") ||
        window.location.hash.includes("kiosk_exit=1"));

    if (isExit) {
      try {
        window.history.replaceState({}, document.title, window.location.pathname);
      } catch (err) {}
      backendService.returnToWailsApp();
      setPendingPin(true);
      return;
    }

    backendService.isPendingPinAuth().then((pending) => {
      if (pending) {
        setPendingPin(true);
      }
    });
  }, []);

  const handlePinSuccess = async () => {
    setPendingPin(false);
    await backendService.completePinAuth(true);
  };

  const handlePinDismiss = async () => {
    setPendingPin(false);
    await backendService.completePinAuth(false);
  };

  return (
    <>
      <div className="min-h-screen w-full flex flex-col items-center justify-start sm:justify-center py-5 px-3.5 sm:p-6 font-sans bg-gray-50 overflow-x-hidden">
        <PrinterList />
        <NetworkPrintingEnabledDialog />
        <NetworkPrinting />
        <SetPinDialog />
      </div>
      {pendingPin && (
        <PINModal
          onSuccess={handlePinSuccess}
          onDismiss={handlePinDismiss}
          title="Security PIN"
          subtitle="Enter PIN to access Wails management"
        />
      )}
    </>
  );
}

function App() {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <WebViewContextWrapper>
          <PINContextWrapper>
            <PrinterContextWrapper>
              <AppContent />
            </PrinterContextWrapper>
          </PINContextWrapper>
        </WebViewContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

export default App;

