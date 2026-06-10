export function isDesktopApp() {
  return typeof window !== 'undefined' && (window.go !== undefined || window.__wails__ !== undefined);
}
