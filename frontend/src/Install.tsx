import { useEffect, useState } from "react";
import { useT } from "./i18n";
import { enablePush, isInApp, isIOS, isStandalone, pushState, type PushState } from "./push";
import { AddPhone } from "./AddPhone";

// InstallBanner: shown on the home screen until the portal runs as an installed
// app with notifications on. Three states, one message each:
//   in-app browser  -> open in your real browser
//   browser         -> install (Android: one tap; iOS: Share -> Add to Home Screen)
//   installed, off  -> enable notifications
export function InstallBanner() {
  const { t } = useT();
  const [dismissed, setDismissed] = useState(() => { try { return sessionStorage.getItem("potok.banner") === "1"; } catch { return false; } });
  const [prompt, setPrompt] = useState<any>(null);
  const [push, setPush] = useState<PushState>("off");
  // Installed-ness is state, not a one-off read: on a laptop the install opens
  // a new window and this tab stays a browser tab, so the banner would sit
  // there until dismissed by hand. `appinstalled` fires here — use it.
  const [standalone, setStandalone] = useState(isStandalone());

  useEffect(() => {
    const onPrompt = (e: Event) => { e.preventDefault(); setPrompt(e); };
    const onInstalled = () => { setPrompt(null); setDismissed(true); };
    const mq = window.matchMedia("(display-mode: standalone)");
    const onMode = () => setStandalone(mq.matches || isStandalone());
    window.addEventListener("beforeinstallprompt", onPrompt);
    window.addEventListener("appinstalled", onInstalled);
    mq.addEventListener?.("change", onMode);
    pushState().then(setPush);
    const onFocus = () => pushState().then(setPush);
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
      window.removeEventListener("beforeinstallprompt", onPrompt);
      window.removeEventListener("appinstalled", onInstalled);
      mq.removeEventListener?.("change", onMode);
    };
  }, []);

  if (dismissed) return null;
  // Nothing left to ask for: installed and subscribed. Also covers the case
  // where the display-mode check fails but push clearly works.
  if (push === "on") return null;
  if (standalone && push !== "off") return null;
  const dismiss = () => { setDismissed(true); try { sessionStorage.setItem("potok.banner", "1"); } catch { /* ignore */ } };

  let body: React.ReactNode;
  if (!standalone && isInApp()) {
    body = <><strong>{t("You are inside WhatsApp's browser.")}</strong> {t("Open this page in Chrome or Safari (menu ⋮ → open in browser), then add it to your home screen. Otherwise the device forgets you.")}</>;
  } else if (!standalone && prompt) {
    body = <><strong>{t("Install the village on this device")}</strong> — {t("one tap, then it opens like an app.")} <button className="primary" onClick={() => prompt.prompt()}>📲 {t("Install")}</button><Signin /></>;
  } else if (!standalone && isIOS()) {
    body = <><strong>{t("Install the village on this device")}</strong>: {t("tap Share")} {t("below, then")} <em>{t("Add to Home Screen")}</em>.<Signin /></>;
  } else if (!standalone) {
    body = <><strong>{t("Install the village on this device")}</strong>: {t("browser menu → Install app / Add to Home Screen.")}<Signin /></>;
  } else {
    body = <><strong>{t("Ring the bell for you too?")}</strong> {t("Get a notification when a house posts, needs something, drives to town, adds an event or goes away.")} <button className="primary" onClick={() => enablePush().catch(() => undefined).then(() => pushState()).then((s) => { setPush(s); if (s !== "off") setDismissed(true); })}>🔔 {t("Enable notifications")}</button></>;
  }
  return (
    <div className="card banner">
      <div className="body">{body}</div>
      <button className="ghost dismiss" aria-label={t("Dismiss")} onClick={dismiss}>✕</button>
    </div>
  );
}

// The installed app starts with empty storage on iOS, so hand the villager a
// code before they walk away from the signed-in browser tab.
function Signin() {
  const { t } = useT();
  return (
    <div style={{ marginTop: ".4rem" }}>
      <span className="small">{t("The installed icon starts signed out. Take a code with you:")} </span>
      <AddPhone compact />
    </div>
  );
}
