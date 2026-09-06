// Install + push plumbing. Everything here degrades to "not available" quietly.
import { api } from "./api";

const lang = () => { try { return localStorage.getItem("potok.lang") === "en" ? "en" : "sl"; } catch { return "sl"; } };

// A desktop PWA window does not always report `standalone` — Chrome may report
// `minimal-ui` or `window-controls-overlay`. Any of them means installed.
const INSTALLED_MODES = ["standalone", "minimal-ui", "window-controls-overlay", "fullscreen"];
export const isStandalone = () =>
  INSTALLED_MODES.some((m) => window.matchMedia(`(display-mode: ${m})`).matches) || (navigator as any).standalone === true;
export const isIOS = () => /iphone|ipad|ipod/i.test(navigator.userAgent);
// WhatsApp's Android in-app browser identifies itself; iOS's does not.
export const isInApp = () => /\bwv\b|WhatsApp|FBAN|FBAV|Instagram/i.test(navigator.userAgent);

export const pushSupported = () => "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;

export async function registerSW(): Promise<ServiceWorkerRegistration | null> {
  if (!("serviceWorker" in navigator)) return null;
  try {
    const base = import.meta.env.BASE_URL;
    const reg = await navigator.serviceWorker.register(base + "sw.js", { scope: base });
    try { await navigator.storage?.persist?.(); } catch { /* optional */ }
    return reg;
  } catch { return null; }
}

function b64ToUint8(b64: string) {
  const pad = "=".repeat((4 - (b64.length % 4)) % 4);
  const raw = atob((b64 + pad).replace(/-/g, "+").replace(/_/g, "/"));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

export type PushState = "unsupported" | "denied" | "off" | "on";

export async function pushState(): Promise<PushState> {
  if (!pushSupported()) return "unsupported";
  if (Notification.permission === "denied") return "denied";
  // `getRegistration` can answer before the worker is ready and report no
  // subscription on a phone that has one — which put the banner back after the
  // villager had already said yes. Wait for the worker, then ask.
  const reg = await Promise.race([
    navigator.serviceWorker.ready,
    new Promise<undefined>((r) => setTimeout(() => r(undefined), 4000)),
  ]).catch(() => undefined) || (await navigator.serviceWorker.getRegistration(import.meta.env.BASE_URL));
  const sub = await reg?.pushManager.getSubscription().catch(() => null);
  if (sub) return "on";
  // Permission granted but no subscription: the origin changed, or the
  // subscription expired. Offer to enable, do not claim it is on.
  return "off";
}

// enablePush asks permission, subscribes this phone, and tells the backend.
export async function enablePush(): Promise<PushState> {
  if (!pushSupported()) return "unsupported";
  const perm = await Notification.requestPermission();
  if (perm !== "granted") return perm === "denied" ? "denied" : "off";
  const reg = (await navigator.serviceWorker.getRegistration(import.meta.env.BASE_URL)) || (await registerSW());
  if (!reg) return "unsupported";
  const { key } = await api<{ key: string }>("/push/key");
  const sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: b64ToUint8(key) });
  await api("/push/subscribe", { method: "POST", body: { ...sub.toJSON(), lang: lang() } });
  return "on";
}

// syncPushLang re-registers this phone when the villager switched SL/EN after
// subscribing — the notification text is built on the server.
export async function syncPushLang() {
  try {
    if ((await pushState()) !== "on") return;
    const sent = localStorage.getItem("potok.pushlang");
    if (sent === lang()) return;
    const reg = await navigator.serviceWorker.getRegistration(import.meta.env.BASE_URL);
    const sub = await reg?.pushManager.getSubscription();
    if (!sub) return;
    await api("/push/subscribe", { method: "POST", body: { ...sub.toJSON(), lang: lang() } });
    localStorage.setItem("potok.pushlang", lang());
  } catch { /* best effort */ }
}

export async function disablePush(): Promise<PushState> {
  const reg = await navigator.serviceWorker.getRegistration(import.meta.env.BASE_URL);
  const sub = await reg?.pushManager.getSubscription();
  if (sub) {
    await api("/push/subscribe", { method: "DELETE", body: { endpoint: sub.endpoint } }).catch(() => {});
    await sub.unsubscribe();
  }
  return "off";
}
