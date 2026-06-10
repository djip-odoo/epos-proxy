let cachedUrl = null;
try {
  cachedUrl = localStorage.getItem('connector_url');
} catch (e) {
  // LocalStorage not available
}

async function checkServer(url) {
  try {
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), 1000);
    const res = await fetch(`${url}/api/ping`, { signal: controller.signal });
    clearTimeout(id);
    return res.ok;
  } catch (e) {
    return false;
  }
}

export async function findConnectorUrl() {
  return window.location.origin;
}

export class HttpConnector {
  constructor() {
    this.baseUrl = null;
    this.discovering = null;
    this.token = null;
  }

  async getBaseUrl(forceRefresh = false) {
    if (this.baseUrl && !forceRefresh) {
      return this.baseUrl;
    }
    if (this.discovering && !forceRefresh) {
      return this.discovering;
    }
    this.discovering = findConnectorUrl().then(url => {
      this.baseUrl = url;
      this.discovering = null;
      return url;
    }).catch(err => {
      this.discovering = null;
      throw err;
    });
    return this.discovering;
  }

  async request(path, options = {}) {
    let url = await this.getBaseUrl();
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    };
    if (this.token) {
      headers['X-LAN-Token'] = this.token;
    }

    const runFetch = async (currentUrl) => {
      const res = await fetch(`${currentUrl}${path}`, {
        ...options,
        headers
      });
      if (!res.ok) {
        let errBody = {};
        try {
          errBody = await res.json();
        } catch (e) { }
        throw {
          code: errBody.code || 'HTTP_ERROR',
          message: errBody.message || `Request failed with status ${res.status}`
        };
      }
      return res.json();
    };

    try {
      return await runFetch(url);
    } catch (err) {
      const isNetworkError = err instanceof TypeError || err.message === 'Failed to fetch' || err.name === 'AbortError' || (err.code === 'HTTP_ERROR' && err.message.includes('Failed to fetch'));
      if (isNetworkError) {
        try {
          url = await this.getBaseUrl(true);
          return await runFetch(url);
        } catch (retryErr) {
          throw this.wrapError(retryErr);
        }
      }
      throw this.wrapError(err);
    }
  }

  wrapError(err) {
    if (err && err.code) {
      return err;
    }
    if (err && typeof err === 'object') {
      return {
        code: 'HTTP_ERROR',
        message: err.message || JSON.stringify(err)
      };
    }
    return {
      code: 'HTTP_ERROR',
      message: String(err)
    };
  }

  async status() {
    return this.request('/api/status');
  }

  async addLANPrinter(ip) {
    return this.request('/api/printers/lan', {
      method: 'POST',
      body: JSON.stringify({ ip })
    });
  }

  async confirmRemoveLANPrinter(ip) {
    const ok = window.confirm(`Are you sure you want to remove the printer at ${ip}?`);
    if (!ok) {
      return false;
    }
    await this.request(`/api/printers/lan?ip=${encodeURIComponent(ip)}`, {
      method: 'DELETE'
    });
    return true;
  }

  async checkLANPrinterStatus(ip) {
    const res = await this.request(`/api/printers/lan/status?ip=${encodeURIComponent(ip)}`);
    return res.online;
  }

  async getLANSettings() {
    return this.request('/api/settings/lan');
  }

  async enableLANAccess() {
    return this.request('/api/settings/lan/enable', { method: 'POST' });
  }

  async disableLANAccess() {
    return this.request('/api/settings/lan/disable', { method: 'POST' });
  }

  async isAutostartEnabled() {
    const res = await this.request('/api/settings/autostart');
    return res.enabled;
  }

  async enableAutostart() {
    return this.request('/api/settings/autostart/enable', { method: 'POST' });
  }

  async disableAutostart() {
    return this.request('/api/settings/autostart/disable', { method: 'POST' });
  }

  async getPrinterSetting(id) {
    return this.request(`/api/settings/printer/${encodeURIComponent(id)}`);
  }

  async setPrinterSetting(id, width, bottomPadding, protocol, cashDrawerPin) {
    return this.request('/api/settings/printer', {
      method: 'POST',
      body: JSON.stringify({
        id,
        width,
        bottom_padding: bottomPadding,
        protocol,
        cash_drawer_pin: cashDrawerPin
      })
    });
  }

  async downloadLogs() {
    try {
      const url = await this.getBaseUrl();
      const downloadUrl = `${url}/api/logs/download`;
      const a = document.createElement('a');
      a.href = downloadUrl;
      a.download = `epos-proxy-logs-${new Date().toISOString().slice(0, 10)}.zip`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    } catch (err) {
      throw this.wrapError(err);
    }
  }

  async browserOpenURL(url) {
    window.open(url, '_blank');
  }

  async setLANPin(pin) {
    throw {
      code: 'UNAUTHORIZED',
      message: 'PIN can only be configured from the desktop application'
    };
  }

  async verifyPin(pin) {
    return this.request('/api/verify-pin', {
      method: 'POST',
      body: JSON.stringify({ pin })
    });
  }
}
