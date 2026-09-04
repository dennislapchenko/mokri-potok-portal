import { useEffect, useState } from "react";
import { useT } from "./i18n";
import { enablePush, isInApp, isIOS, isStandalone, pushState, type PushState } from "./push";

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
  const standalone = isStandalone();

  useEffect(() => {
    const h = (e: Event) => { e.preventDefault(); setPrompt(e); };
    window.addEventListener("beforeinstallprompt", h);
    pushState().then(setPush);
    return () => window.removeEventListener("beforeinstallprompt", h);
  }, []);

  if (dismissed) return null;
  if (standalone && push !== "off") return null;
  const dismiss = () => { setDismissed(true); try { sessionStorage.setItem("potok.banner", "1"); } catch { /* ignore */ } };

  let body: React.ReactNode;
  if (!standalone && isInApp()) {
    body = <><strong>{t("You are inside WhatsApp's browser.")}</strong> {t("Open this page in Chrome or Safari (menu ⋮ → open in browser), then add it to your home screen. Otherwise the phone forgets you.")}</>;
  } else if (!standalone && prompt) {
    body = <><strong>{t("Install the village on this phone")}</strong> — {t("one tap, then it opens like an app and remembers you.")} <button className="primary" onClick={() => prompt.prompt()}>📲 {t("Install")}</button></>;
  } else if (!standalone && isIOS()) {
    body = <><strong>{t("Install the village on this phone")}</strong>: {t("tap Share")} <span aria-hidden="true">⎋</span> {t("below, then")} <em>{t("Add to Home Screen")}</em>. {t("Then it opens like an app and remembers you.")}</>;
  } else if (!standalone) {
    body = <><strong>{t("Install the village on this phone")}</strong>: {t("browser menu → Install app / Add to Home Screen.")} {t("Then it opens like an app and remembers you.")}</>;
  } else {
    body = <><strong>{t("Ring the bell for you too?")}</strong> {t("Get a notification when a house posts, needs something, drives to town, adds an event or goes away.")} <button className="primary" onClick={() => enablePush().then(setPush)}>🔔 {t("Enable notifications")}</button></>;
  }
  return (
    <div className="card banner">
      <div className="body">{body}</div>
      <button className="ghost dismiss" aria-label={t("Dismiss")} onClick={dismiss}>✕</button>
    </div>
  );
}
