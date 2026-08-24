import { createContext, useCallback, useEffect, useState } from "react";
import {
  GetWebViewConfig,
  SetWebViewEnabled,
  SetWebViewPIN,
  SetWebViewURL,
  SetWindowFullscreen,
  ValidateWebViewPIN,
} from "../../wailsjs/go/main/App";

export type WebViewConfig = {
  url: string;
  enabled: boolean;
  hasPIN: boolean;
};

type WebViewContextType = {
  data: {
    config: WebViewConfig | null;
    isKioskActive: boolean;
  };
  actions: {
    saveURL: (url: string) => Promise<void>;
    savePIN: (pin: string) => Promise<void>;
    toggleEnabled: (v: boolean) => Promise<void>;
    validatePIN: (pin: string) => Promise<boolean>;
    exitKiosk: () => Promise<void>;
    enterKiosk: () => Promise<void>;
  };
};

export const WebViewContext = createContext({} as WebViewContextType);

interface WebViewContextWrapperProps {
  children: React.ReactNode;
}

export const WebViewContextWrapper = ({
  children,
}: WebViewContextWrapperProps) => {
  const [config, setConfig] = useState<WebViewConfig | null>(null);
  const [isKioskActive, setIsKioskActive] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const cfg = await GetWebViewConfig();
      setConfig(cfg);
      // If kiosk was enabled on startup, activate the overlay immediately
      if (cfg.enabled && cfg.url) {
        setIsKioskActive(true);
        await SetWindowFullscreen(true);
      }
    } catch (err) {
      console.error("Failed to fetch WebView config:", err);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const saveURL = async (url: string) => {
    await SetWebViewURL(url);
    await refresh();
  };

  const savePIN = async (pin: string) => {
    await SetWebViewPIN(pin);
    await refresh();
  };

  const toggleEnabled = async (v: boolean) => {
    await SetWebViewEnabled(v);
    await refresh();
    if (v && config?.url) {
      await enterKiosk();
    } else if (!v) {
      await exitKiosk();
    }
  };

  const validatePIN = async (pin: string): Promise<boolean> => {
    return ValidateWebViewPIN(pin);
  };

  const enterKiosk = async () => {
    setIsKioskActive(true);
    await SetWindowFullscreen(true);
  };

  const exitKiosk = async () => {
    setIsKioskActive(false);
    await SetWindowFullscreen(false);
  };

  return (
    <WebViewContext.Provider
      value={{
        data: { config, isKioskActive },
        actions: {
          saveURL,
          savePIN,
          toggleEnabled,
          validatePIN,
          enterKiosk,
          exitKiosk,
        },
      }}
    >
      {children}
    </WebViewContext.Provider>
  );
};
