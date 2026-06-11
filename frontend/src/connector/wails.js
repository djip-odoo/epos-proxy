import * as App from '../../wailsjs/go/main/App.js';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime.js';

export class WailsConnector {
  async wrap(fn, ...args) {
    try {
      return await fn(...args);
    } catch (err) {
      if (err && typeof err === 'object') {
        throw {
          code: err.code || 'DESKTOP_ERROR',
          message: err.message || JSON.stringify(err)
        };
      }
      throw {
        code: 'DESKTOP_ERROR',
        message: String(err)
      };
    }
  }

  async status() {
    return this.wrap(App.Status);
  }

  async addLANPrinter(ip) {
    return this.wrap(App.AddLANPrinter, ip);
  }

  async confirmRemoveLANPrinter(ip) {
    return this.wrap(App.ConfirmRemoveLANPrinter, ip);
  }

  async checkLANPrinterStatus(ip) {
    return this.wrap(App.CheckLANPrinterStatus, ip);
  }

  async getLANSettings() {
    return this.wrap(App.GetLANSettings);
  }

  async enableLANAccess() {
    return this.wrap(App.EnableLANAccess);
  }

  async disableLANAccess() {
    return this.wrap(App.DisableLANAccess);
  }

  async isAutostartEnabled() {
    return this.wrap(App.IsAutostartEnabled);
  }

  async enableAutostart() {
    return this.wrap(App.EnableAutostart);
  }

  async disableAutostart() {
    return this.wrap(App.DisableAutostart);
  }

  async getPrinterSetting(id) {
    return this.wrap(App.GetPrinterSetting, id);
  }

  async setPrinterSetting(id, width, bottomPadding, protocol, cashDrawerPin) {
    return this.wrap(App.SetPrinterSetting, id, width, bottomPadding, protocol, cashDrawerPin);
  }

  async downloadLogs() {
    return this.wrap(App.DownloadLogs);
  }

  async browserOpenURL(url) {
    return this.wrap(BrowserOpenURL, url);
  }

  async setLANPin(pin) {
    return this.wrap(App.SetLANPin, pin);
  }

  async verifyPin(pin) {
    return { token: 'wails-internal' };
  }

  // ── Customer Display WebView ───────────────────────────────────────────────

  async getCustomerDisplayURLs() {
    return this.wrap(App.GetCustomerDisplayURLs);
  }

  async getActiveCustomerDisplayURL() {
    return this.wrap(App.GetActiveCustomerDisplayURL);
  }

  async addCustomerDisplayURL(name, url, description) {
    return this.wrap(App.AddCustomerDisplayURL, name, url, description);
  }

  async updateCustomerDisplayURL(id, name, url, description, enabled) {
    return this.wrap(App.UpdateCustomerDisplayURL, id, name, url, description, enabled);
  }

  async setActiveCustomerDisplayURL(id) {
    return this.wrap(App.SetActiveCustomerDisplayURL, id);
  }

  async disableCustomerDisplayURL(id) {
    return this.wrap(App.DisableCustomerDisplayURL, id);
  }

  async deleteCustomerDisplayURL(id) {
    return this.wrap(App.DeleteCustomerDisplayURL, id);
  }

  async validateAdminPin(pin) {
    return this.wrap(App.ValidateAdminPin, pin);
  }

  async setWindowFullscreen(fullscreen) {
    return this.wrap(App.SetWindowFullscreen, fullscreen);
  }

  async isCustomerDisplayOpen() {
    return this.wrap(App.IsCustomerDisplayOpen);
  }

  async setCustomerDisplayOpen(open) {
    return this.wrap(App.SetCustomerDisplayOpen, open);
  }
}
