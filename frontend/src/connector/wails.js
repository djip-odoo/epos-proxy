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
}
