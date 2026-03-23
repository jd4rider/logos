'use strict';

export function EventsOn(eventName, callback) {
  window.runtime?.EventsOn(eventName, callback);
}

export function EventsEmit(eventName, ...data) {
  window.runtime?.EventsEmit(eventName, ...data);
}

export function EventsOff(...eventNames) {
  window.runtime?.EventsOff(...eventNames);
}

export function WindowMinimise() {
  window.runtime?.WindowMinimise();
}

export function WindowMaximise() {
  window.runtime?.WindowMaximise();
}

export function WindowClose() {
  window.runtime?.WindowClose();
}

export function BrowserOpenURL(url) {
  window.runtime?.BrowserOpenURL(url);
}
