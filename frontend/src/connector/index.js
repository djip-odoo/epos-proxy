import { isDesktopApp } from './env.js';
import { WailsConnector } from './wails.js';
import { HttpConnector } from './http.js';

export const connector = isDesktopApp() ? new WailsConnector() : new HttpConnector();
export { isDesktopApp } from './env.js';
export { safeEventsOn } from './events.js';
