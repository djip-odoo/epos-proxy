import { useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { WebViewContext } from "../contexts/WebViewContext";

const MAX_ATTEMPTS = 3;
const COOLDOWN_SECONDS = 30;

interface PINModalProps {
  onSuccess: () => void;
  onDismiss: () => void;
}

export default function PINModal({ onSuccess, onDismiss }: PINModalProps) {
  const { actions } = useContext(WebViewContext);
  const [digits, setDigits] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [attempts, setAttempts] = useState(0);
  const [cooldown, setCooldown] = useState(0);
  const [shaking, setShaking] = useState(false);
  const cooldownRef = useRef<number | null>(null);

  // keyboard listener
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (cooldown > 0) return;
      if (e.key >= "0" && e.key <= "9") {
        addDigit(e.key);
      } else if (e.key === "Backspace") {
        removeDigit();
      } else if (e.key === "Escape") {
        onDismiss();
      }
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [digits, cooldown, onDismiss]);

  // auto-submit when 4 digits entered
  useEffect(() => {
    if (digits.length === 4 && cooldown === 0) {
      submit(digits.join(""));
    }
  }, [digits]);

  const addDigit = (d: string) => {
    if (digits.length >= 4 || cooldown > 0) return;
    setDigits((prev) => [...prev, d]);
    setError(null);
  };

  const removeDigit = () => {
    setDigits((prev) => prev.slice(0, -1));
    setError(null);
  };

  const shake = () => {
    setShaking(true);
    setTimeout(() => setShaking(false), 500);
  };

  const startCooldown = () => {
    setCooldown(COOLDOWN_SECONDS);
    let remaining = COOLDOWN_SECONDS;
    cooldownRef.current = window.setInterval(() => {
      remaining -= 1;
      setCooldown(remaining);
      if (remaining <= 0) {
        clearInterval(cooldownRef.current!);
        setAttempts(0);
        setError(null);
      }
    }, 1000);
  };

  const submit = async (pin: string) => {
    const ok = await actions.validatePIN(pin);
    if (ok) {
      onSuccess();
      return;
    }

    shake();
    setDigits([]);
    const next = attempts + 1;
    setAttempts(next);
    if (next >= MAX_ATTEMPTS) {
      setError(`Too many attempts. Wait ${COOLDOWN_SECONDS}s.`);
      startCooldown();
    } else {
      setError(`Incorrect PIN (${MAX_ATTEMPTS - next} attempt${MAX_ATTEMPTS - next === 1 ? "" : "s"} left)`);
    }
  };

  const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "", "0", "⌫"];

  return createPortal(
    <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80 backdrop-blur-sm">
      <div
        className={`bg-white rounded-3xl shadow-2xl p-8 w-72 flex flex-col items-center gap-6 select-none ${
          shaking ? "animate-[shake_0.5s_ease-in-out]" : ""
        }`}
        style={shaking ? { animation: "shake 0.5s ease-in-out" } : {}}
      >
        {/* Title */}
        <div className="text-center">
          <div className="text-2xl font-semibold text-gray-800 mb-1">
            Enter PIN
          </div>
          <div className="text-sm text-gray-400">
            4-digit unlock code
          </div>
        </div>

        {/* Dot display */}
        <div className="flex gap-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className={`w-4 h-4 rounded-full border-2 transition-all duration-150 ${
                i < digits.length
                  ? "bg-odoo border-odoo scale-110"
                  : "border-gray-300 bg-transparent"
              }`}
            />
          ))}
        </div>

        {/* Error / cooldown */}
        {error && (
          <div className="text-sm text-red-500 text-center -mt-2">
            {cooldown > 0 ? `${error.split(".")[0]}. Wait ${cooldown}s.` : error}
          </div>
        )}

        {/* Numpad */}
        <div className="grid grid-cols-3 gap-3 w-full">
          {keys.map((k, i) => {
            if (k === "") return <div key={i} />;

            const isDelete = k === "⌫";
            const disabled = cooldown > 0 || (digits.length >= 4 && !isDelete);

            return (
              <button
                key={i}
                disabled={disabled}
                onClick={() => (isDelete ? removeDigit() : addDigit(k))}
                className={`h-14 rounded-2xl text-lg font-medium transition-all duration-100 cursor-pointer
                  ${
                    isDelete
                      ? "bg-gray-100 text-gray-500 hover:bg-gray-200 active:scale-95"
                      : "bg-gray-50 text-gray-800 hover:bg-odoo/10 hover:text-odoo active:scale-95 active:bg-odoo/20"
                  }
                  disabled:opacity-40 disabled:cursor-not-allowed`}
              >
                {k}
              </button>
            );
          })}
        </div>

        {/* Cancel */}
        <button
          onClick={onDismiss}
          className="text-sm text-gray-400 hover:text-gray-600 cursor-pointer transition-colors"
        >
          Cancel
        </button>
      </div>

      <style>{`
        @keyframes shake {
          0%,100%{transform:translateX(0)}
          15%{transform:translateX(-8px)}
          30%{transform:translateX(8px)}
          45%{transform:translateX(-6px)}
          60%{transform:translateX(6px)}
          75%{transform:translateX(-3px)}
          90%{transform:translateX(3px)}
        }
      `}</style>
    </div>,
    document.body,
  );
}
