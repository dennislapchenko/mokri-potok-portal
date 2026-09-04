// Install + push plumbing. Everything here degrades to "not available" quietly.
import { api } from "./api";

export const isStandalone = () =>
  window.matchMedia("(display-mode: standalone)").matches || (navigator as any).standalone === true;
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
  const reg = await navigator.serviceWorker.getRegistration(import.meta.env.BASE_URL);
  const sub = await reg?.pushManager.getSubscription();
  return sub ? "on" : "off";
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
  await api("/push/subscribe", { method: "POST", body: sub.toJSON() });
  return "on";
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
