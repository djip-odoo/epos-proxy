import PrinterList from "./components/PrinterList";
import { AppContextWrapper } from "./contexts/AppContext";
import { PrinterContextWrapper } from "./contexts/PrinterContext";
import { ToastContextWrapper } from "./contexts/ToastContext";

function App() {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <PrinterContextWrapper>
          <div className="min-h-screen flex flex-col items-center justify-center p-4 sm:p-6 font-sans bg-gray-50">
            <PrinterList />
          </div>
        </PrinterContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

export default App;
