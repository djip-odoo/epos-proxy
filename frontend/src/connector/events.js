import { isDesktopApp } from './env.js';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';

export function safeEventsOn(eventName, callback) {
  if (isDesktopApp()) {
    try {
      return EventsOn(eventName, callback);
    } catch (e) {
      console.warn('Wails EventsOn failed:', e);
    }
  }
}
