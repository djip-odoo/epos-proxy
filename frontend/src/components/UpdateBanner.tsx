import { useCallback, useEffect, useRef, useState } from "react";
import { ApplyUpdate, CheckForUpdate, DownloadUpdate } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

interface ProgressData {
  downloaded: number;
  total: number;
}

export default function UpdateBanner() {
  const [info, setInfo] = useState<main.UpdateInfo | null>(null);
  const [checking, setChecking] = useState(true);
  const [downloading, setDownloading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [progress, setProgress] = useState(0);
  const applyingRef = useRef(false);

  useEffect(() => {
    let cancelled = false;

    const unsubscribe = EventsOn("update-progress", (data: ProgressData) => {
      if (data && data.total > 0) {
        setProgress(Math.round((data.downloaded / data.total) * 100));
      }
    });

    CheckForUpdate()
      .then((result) => {
        if (!cancelled && result && result.checkOk) {
          setInfo(result);
        }
      })
      .catch((err) => console.error("Failed to check for updates", err))
      .finally(() => {
        if (!cancelled) {
          setChecking(false);
        }
      });

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  const handleUpdate = useCallback(async () => {
    if (applyingRef.current) {
      return;
    }
    applyingRef.current = true;
    setDownloading(true);
    try {
      await DownloadUpdate();
      setDownloading(false);
      setApplying(true);
      // ApplyUpdate replaces the binary and restarts the app.
      await ApplyUpdate();
    } catch (err) {
      applyingRef.current = false;
      setDownloading(false);
      setApplying(false);
      console.error("Update failed", err);
    }
  }, []);

  if (checking || !info || !info.available) {
    return null;
  }

  if (applying) {
    return (
      <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
        <p className="text-sm font-semibold text-gray-800">
          Installing version {info.latestVersion}
        </p>
        <p className="text-xs text-gray-500 mt-0.5">
          The app will restart with the new version shortly.
        </p>
      </div>
    );
  }

  return (
    <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
      <div>
        <p className="text-sm font-semibold text-gray-800">
          ePOS Proxy {info.latestVersion} is available
        </p>
        <p className="text-xs text-gray-500 mt-0.5">
          You are running version {info.currentVersion}.
        </p>
      </div>

      {info.notes && info.notes.length > 0 && (
        <p className="text-xs text-gray-600 mt-2 whitespace-pre-line line-clamp-3">
          {info.notes}
        </p>
      )}

      {downloading ? (
        <div className="mt-3">
          <div className="h-2 rounded-full bg-gray-200 overflow-hidden">
            <div
              className="h-full bg-odoo transition-all duration-200"
              style={{ width: `${Math.max(progress, 2)}%` }}
            />
          </div>
          <p className="text-xs text-gray-500 mt-1">{progress}% downloaded</p>
        </div>
      ) : (
        <button
          type="button"
          onClick={handleUpdate}
          className="mt-3 w-full px-4 py-2 rounded-lg bg-odoo text-white text-sm font-medium hover:opacity-90 transition-opacity"
        >
          Update Now
        </button>
      )}
    </div>
  );
}