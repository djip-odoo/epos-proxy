import { useState } from "react";
import Dialog from "./Dialog";
import { useClipboard } from "../hooks/useClipboard";

interface CodeSnippetProps {
  label: string;
  command: string;
  description?: string;
}

function CodeSnippet({ label, command, description }: CodeSnippetProps) {
  const { copied, copy } = useClipboard({
    successMessage: `${label} copied to clipboard`,
  });

  return (
    <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-800">{label}</span>
        <button
          type="button"
          onClick={() => copy(command)}
          className="
            inline-flex items-center gap-1 rounded-md border border-gray-300
            bg-white px-2 py-1 text-[11px] font-medium text-gray-700
            shadow-2xs transition-colors hover:border-gray-400 hover:bg-gray-50
            cursor-pointer
          "
        >
          {copied ? (
            <>
              <svg
                className="h-3 w-3 text-green-600"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M5 13l4 4L19 7"
                />
              </svg>
              <span className="text-green-600 font-medium">Copied</span>
            </>
          ) : (
            <>
              <svg
                className="h-3 w-3 text-gray-500"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <rect
                  x="8"
                  y="8"
                  width="12"
                  height="12"
                  rx="2"
                  strokeWidth="2"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M16 8V6a2 2 0 00-2-2H6a2 2 0 00-2 2v8a2 2 0 002 2h2"
                />
              </svg>
              <span>Copy</span>
            </>
          )}
        </button>
      </div>

      {description && (
        <p className="mt-1 text-[11px] text-gray-500">{description}</p>
      )}

      <div className="mt-2 overflow-x-auto rounded-lg bg-gray-900 px-3 py-2 text-[11px] font-mono text-green-400">
        <code>{command}</code>
      </div>
    </div>
  );
}

export default function TroubleshootDialog() {
  const [activeTab, setActiveTab] = useState<"service" | "edge" | "faq">("service");

  const fullServiceScript = `:: Create the Windows Service
sc.exe create odoopos binPath= "C:\\Program Files\\ePOS proxy\\ePOS proxy\\ePOS proxy.exe" start= auto obj= "LocalSystem"

:: Start the service
sc.exe start odoopos

:: Query status (should report STATE: 4 RUNNING)
sc.exe query odoopos`;

  const { copied: copiedAll, copy: copyAll } = useClipboard({
    successMessage: "All service setup commands copied",
  });

  return (
    <Dialog
      title="Troubleshoot & Setup Guide"
      actions={[
        {
          name: "close",
          label: "Close",
          variant: "secondary",
        },
      ]}
      openButton={
        <button
          type="button"
          className="
            w-full flex items-center justify-between
            rounded-xl border border-gray-200 bg-white hover:bg-gray-50
            px-4 py-3 transition-colors cursor-pointer shadow-2xs
          "
        >
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-blue-50 text-blue-600">
              <svg
                className="h-5 w-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
            </div>
            <div className="text-left">
              <div className="text-sm font-semibold text-gray-800">
                Troubleshoot & Kiosk Setup
              </div>
              <div className="text-xs text-gray-500">
                Windows Service commands and Edge Kiosk guide
              </div>
            </div>
          </div>
          <span className="rounded-full bg-blue-50 border border-blue-200 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide text-blue-600">
            Guide
          </span>
        </button>
      }
    >
      <div className="flex flex-col gap-5 text-gray-700">
        {/* Navigation Tabs */}
        <div className="flex rounded-lg bg-gray-100 p-1 text-xs font-medium">
          <button
            type="button"
            onClick={() => setActiveTab("service")}
            className={`
              flex-1 rounded-md py-1.5 transition-colors cursor-pointer text-center
              ${activeTab === "service"
                ? "bg-white text-gray-900 shadow-2xs font-semibold"
                : "text-gray-600 hover:text-gray-900"
              }
            `}
          >
            Windows Service
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("edge")}
            className={`
              flex-1 rounded-md py-1.5 transition-colors cursor-pointer text-center
              ${activeTab === "edge"
                ? "bg-white text-gray-900 shadow-2xs font-semibold"
                : "text-gray-600 hover:text-gray-900"
              }
            `}
          >
            Edge Kiosk Guide
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("faq")}
            className={`
              flex-1 rounded-md py-1.5 transition-colors cursor-pointer text-center
              ${activeTab === "faq"
                ? "bg-white text-gray-900 shadow-2xs font-semibold"
                : "text-gray-600 hover:text-gray-900"
              }
            `}
          >
            Diagnostics
          </button>
        </div>

        {/* TAB 1: WINDOWS SERVICE */}
        {activeTab === "service" && (
          <div className="flex flex-col gap-4">
            <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-3 text-xs text-amber-900">
              <div className="flex items-center gap-1.5 font-semibold">
                <svg
                  className="h-4 w-4 shrink-0 text-amber-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                Administrator Prompt Required
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-amber-800">
                Open <strong>Command Prompt</strong> or <strong>PowerShell</strong> as <strong>Administrator</strong> to register and start the service.
              </p>
            </div>

            <div className="flex items-center justify-between">
              <span className="text-xs font-bold uppercase tracking-wider text-gray-500">
                Quick Setup Commands
              </span>
              <button
                type="button"
                onClick={() => copyAll(fullServiceScript)}
                className="
                  inline-flex items-center gap-1 rounded-lg bg-odoo px-2.5 py-1
                  text-[11px] font-medium text-white shadow-2xs transition-colors
                  hover:bg-odoo-dark cursor-pointer
                "
              >
                {copiedAll ? "Copied All!" : "Copy All Commands"}
              </button>
            </div>

            <div className="flex flex-col gap-3">
              <CodeSnippet
                label="1. Create Windows Service"
                description="Registers ePOS proxy as an auto-start service under LocalSystem."
                command='sc.exe create odoopos binPath= "C:\Program Files\ePOS proxy\ePOS proxy\ePOS proxy.exe" start= auto obj= "LocalSystem"'
              />

              <CodeSnippet
                label="2. Start Service"
                description="Starts the background server immediately without rebooting."
                command="sc.exe start odoopos"
              />

              <CodeSnippet
                label="3. Query Status"
                description="Verifies the service state reports STATE : 4 RUNNING."
                command="sc.exe query odoopos"
              />
            </div>

            <div className="mt-1 border-t border-gray-200 pt-3">
              <span className="text-xs font-bold uppercase tracking-wider text-gray-500">
                Service Management
              </span>
              <div className="mt-2.5 grid grid-cols-1 sm:grid-cols-2 gap-2">
                <div className="rounded-lg border border-gray-200 bg-gray-50 p-2.5">
                  <div className="text-[11px] font-semibold text-gray-700">Stop Service</div>
                  <div className="mt-1 font-mono text-[10px] text-gray-800">sc.exe stop odoopos</div>
                </div>
                <div className="rounded-lg border border-gray-200 bg-gray-50 p-2.5">
                  <div className="text-[11px] font-semibold text-gray-700">Delete / Uninstall</div>
                  <div className="mt-1 font-mono text-[10px] text-gray-800">sc.exe delete odoopos</div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: EDGE KIOSK GUIDE */}
        {activeTab === "edge" && (
          <div className="flex flex-col gap-4 text-xs">
            {/* Assigned Access */}
            <div className="rounded-xl border border-gray-200 bg-white p-3.5 shadow-2xs">
              <div className="flex items-center gap-2 font-semibold text-gray-900">
                <span className="flex h-5 w-5 items-center justify-center rounded-full bg-odoo/10 text-odoo text-[11px] font-bold">
                  A
                </span>
                Method 1: Windows Assigned Access (Recommended)
              </div>
              <p className="mt-1 text-[11px] text-gray-500">
                Locks down Windows into a dedicated single-app kiosk with automatic login.
              </p>

              <ol className="mt-3 flex flex-col gap-2 list-decimal list-inside text-[11px] text-gray-700">
                <li>
                  Open Windows <strong>Settings</strong> &gt; <strong>Accounts</strong> &gt; <strong>Other users</strong> (or <em>Family & other users</em>).
                </li>
                <li>
                  Under <strong>Set up a kiosk</strong>, select <strong>Assigned access</strong> &gt; <strong>Get started</strong>.
                </li>
                <li>
                  Enter a name for the kiosk account (e.g. <code className="bg-gray-100 px-1 py-0.5 rounded font-mono">KioskUser</code>).
                </li>
                <li>
                  Choose <strong>Microsoft Edge</strong> as the kiosk application.
                </li>
                <li>
                  Select <strong>As a digital sign or interactive display</strong> (full screen, no address bar or tabs).
                </li>
                <li>
                  Enter the POS URL: <code className="bg-gray-100 px-1 py-0.5 rounded font-mono">http://127.0.0.1:4545/</code>
                </li>
                <li>
                  Restart the machine. Windows will sign in to the kiosk account and launch Edge automatically.
                </li>
              </ol>
            </div>

            {/* Shortcut / CLI */}
            <div className="rounded-xl border border-gray-200 bg-white p-3.5 shadow-2xs">
              <div className="flex items-center gap-2 font-semibold text-gray-900">
                <span className="flex h-5 w-5 items-center justify-center rounded-full bg-blue-100 text-blue-700 text-[11px] font-bold">
                  B
                </span>
                Method 2: Command Line / Desktop Shortcut
              </div>
              <p className="mt-1 text-[11px] text-gray-500">
                Launch Microsoft Edge directly in fullscreen kiosk mode on any user profile.
              </p>

              <div className="mt-2.5">
                <CodeSnippet
                  label="Edge Kiosk Launch Command"
                  command='"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --kiosk http://127.0.0.1:4545/ --edge-kiosk-type=fullscreen --no-first-run'
                />
              </div>

              <div className="mt-2 text-[11px] text-gray-500">
                Tip: Press <kbd className="rounded border border-gray-300 bg-gray-100 px-1 py-0.5 font-mono text-[10px]">Alt + F4</kbd> to exit fullscreen Edge kiosk.
              </div>
            </div>
          </div>
        )}

        {/* TAB 3: DIAGNOSTICS & FAQ */}
        {activeTab === "faq" && (
          <div className="flex flex-col gap-3 text-xs">
            <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
              <div className="font-semibold text-gray-900">
                Why was there an "EBWebView access denied" error previously?
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-gray-600">
                When launched under the <code className="font-mono bg-white px-1 py-0.5 rounded">LocalSystem</code> account, services run in Session 0 without an interactive desktop. If Wails attempted to launch Edge WebView2, it failed because the system profile has no writable user data folder. With native Windows Service support, ePOS proxy starts in headless server mode and bypasses WebView2 completely.
              </p>
            </div>

            <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
              <div className="font-semibold text-gray-900">
                What address and port does the proxy listen on?
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-gray-600">
                The service binds to <code className="font-mono bg-white px-1 py-0.5 rounded">0.0.0.0:4545</code> (or up to 4555 if 4545 is occupied). You can verify it is active by opening <a href="http://127.0.0.1:4545/" target="_blank" rel="noreferrer" className="text-odoo font-medium underline">http://127.0.0.1:4545/</a> in any browser.
              </p>
            </div>

            <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
              <div className="font-semibold text-gray-900">
                How do I view service logs?
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-gray-600">
                When running as a service, logs are stored in <code className="font-mono bg-white px-1 py-0.5 rounded">%ProgramData%\EposProxy\logs\epos-proxy.log</code> (typically <code className="font-mono bg-white px-1 py-0.5 rounded">C:\ProgramData\EposProxy\logs</code>).
              </p>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
