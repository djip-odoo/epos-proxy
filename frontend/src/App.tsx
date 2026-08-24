import KioskOverlay from "./components/KioskOverlay";
import PrinterList from "./components/PrinterList";
import { AppContextWrapper } from "./contexts/AppContext";
import { PrinterContextWrapper } from "./contexts/PrinterContext";
import { ToastContextWrapper } from "./contexts/ToastContext";
import { WebViewContextWrapper } from "./contexts/WebViewContext";

function App() {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <WebViewContextWrapper>
          <PrinterContextWrapper>
            {/* Kiosk overlay: renders on top of everything when active */}
            <KioskOverlay />
            <div className="min-h-screen flex flex-col items-center justify-center p-4 sm:p-6 font-sans bg-gray-50">
              <PrinterList />
            </div>
          </PrinterContextWrapper>
        </WebViewContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

export default App;
