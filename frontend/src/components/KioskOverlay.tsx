import { useContext, useEffect, useRef, useState } from "react";
import { WebViewContext } from "../contexts/WebViewContext";
import { AppContext } from "../contexts/AppContext";
import PINModal from "./PINModal";

/** Radius (px) of the circular tap zone at each corner. */
const CORNER_RADIUS = 80;
/** Number of corner taps needed to open the PIN dialog. */
const REQUIRED_TAPS = 4;
/** Milliseconds before the tap counter resets to zero. */
const RESET_MS = 1000;

/**
 * Full-screen iframe overlay that locks the UI into kiosk mode.
 *
 * The iframe absorbs all pointer events — they cannot bubble to the parent
 * window. Four transparent circular zones are layered above the iframe at
 * each corner. Tapping any corner REQUIRED_TAPS times within RESET_MS ms
 * opens the PIN dialog.
 *
 * Kiosk mode ONLY runs in the desktop Wails application, never in the
 * remote webview / admin browser.
 */
export default function KioskOverlay() {
  const appContext = useContext(AppContext);
  const { data, actions } = useContext(WebViewContext);
  const [pinVisible, setPinVisible] = useState(false);

  const tapCount = useRef(0);
  const resetTimer = useRef<number | null>(null);

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "127.0.0.1" ||
      window.location.hostname === "localhost");

  const isKiosk =
    Boolean(data.config?.url) &&
    Boolean(data.config?.enabled) &&
    Boolean(data.isKioskActive) &&
    (Boolean(appContext.data.isWails) || isLocalhost);

  useEffect(() => {
    if (!isKiosk) return;

    const handleContextMenu = (e: MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
    };

    window.addEventListener("contextmenu", handleContextMenu, true);
    document.addEventListener("contextmenu", handleContextMenu, true);
    return () => {
      window.removeEventListener("contextmenu", handleContextMenu, true);
      document.removeEventListener("contextmenu", handleContextMenu, true);
    };
  }, [isKiosk]);

  const handleCornerTap = (e: React.PointerEvent) => {
    if (e.button !== 0) return; // Only trigger on primary click / touch
    if (pinVisible) return;

    tapCount.current += 1;

    if (resetTimer.current !== null) clearTimeout(resetTimer.current);
    resetTimer.current = window.setTimeout(() => {
      tapCount.current = 0;
    }, RESET_MS);

    if (tapCount.current >= REQUIRED_TAPS) {
      tapCount.current = 0;
      if (resetTimer.current !== null) {
        clearTimeout(resetTimer.current);
        resetTimer.current = null;
      }
      setPinVisible(true);
    }
  };

  const handlePINSuccess = async () => {
    setPinVisible(false);
    await actions.exitKiosk();
  };

  const handlePINDismiss = () => {
    setPinVisible(false);
  };

  if (!isKiosk || !data.config?.url) {
    return null;
  }

  const cornerBase: React.CSSProperties = {
    position: "fixed",
    zIndex: 9995,
    width: CORNER_RADIUS,
    height: CORNER_RADIUS,
    borderRadius: "50%",
    opacity: 0,
    cursor: "default",
  };

  return (
    <>
      {/* Top-right corner tap zone */}
      <div
        style={{ ...cornerBase, top: 0, right: 0 }}
        className="select-none"
        onPointerDown={handleCornerTap}
        onContextMenu={(e) => e.preventDefault()}
      />

      {/* Full-screen kiosk iframe */}
      <div
        className="fixed inset-0 bg-black select-none"
        style={{ zIndex: 9990, pointerEvents: pinVisible ? "none" : "auto" }}
        onContextMenu={(e) => e.preventDefault()}
      >
        <iframe
          key={data.reloadNonce}
          src={data.config.url}
          title="Kiosk"
          className="w-full h-full border-0"
          sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-top-navigation"
        />
      </div>

      {/* PIN modal (above everything) */}
      {pinVisible && (
        <PINModal onSuccess={handlePINSuccess} onDismiss={handlePINDismiss} />
      )}
    </>
  );
}
