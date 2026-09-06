import { useEffect, useState } from "react";
import { api } from "./api";
import { useT } from "./i18n";

// A six-digit code that logs one more phone of this house in. It exists because
// an iPhone home-screen app gets storage of its own: installing after signing in
// leaves the new icon signed out, and the invite link is buried in a chat.
export function AddPhone({ compact }: { compact?: boolean }) {
  const { t } = useT();
  const [pin, setPin] = useState<string>("");
  const [until, setUntil] = useState<number>(0);
  const [left, setLeft] = useState<number>(0);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!until) return;
    const id = setInterval(() => setLeft(Math.max(0, Math.round((until - Date.now()) / 1000))), 500);
    return () => clearInterval(id);
  }, [until]);

  const make = async () => {
    setBusy(true);
    try {
      const r = await api<{ code: string; expires_at: string }>("/pair", { method: "POST" });
      setPin(r.code);
      setUntil(new Date(r.expires_at).getTime());
    } finally { setBusy(false); }
  };

  // Six digits read off one device and typed into another is where a villager
  // gives up. The clipboard carries the raw code; the gate strips spaces anyway.
  const copy = () => { navigator.clipboard?.writeText(pin); setCopied(true); setTimeout(() => setCopied(false), 1500); };

  const expired = pin !== "" && left === 0 && until !== 0 && Date.now() > until;
  if (!pin || expired) {
    return <button disabled={busy} onClick={make}>➕ {t("Add another device")}</button>;
  }
  const mm = Math.floor(left / 60), ss = String(left % 60).padStart(2, "0");
  return (
    <div>
      <div className="pin">{pin.slice(0, 3)} {pin.slice(3)}</div>
      <div style={{ textAlign: "center" }}>
        <button className="lesser" onClick={copy}>📋 {copied ? t("Copied") : t("Copy")}</button>
      </div>
      <p className="small">
        {t("On the other device open the portal and type this code. It works once, and only for")} {mm}:{ss}.
        {!compact && <> {t("This is how a second device of the same house gets in, and how you sign in again after adding the portal to your home screen.")}</>}
      </p>
    </div>
  );
}
