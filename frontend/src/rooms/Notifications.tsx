import { useEffect, useState } from "react";
import { api } from "../api";
import { useT } from "../i18n";
import { disablePush, enablePush, pushState, type PushState } from "../push";

// Notifications: this phone on/off, and the house-wide list of kinds. All kinds
// are on until a house switches one off (the backend stores only the off list).
export function Notifications() {
  const { t } = useT();
  const [state, setState] = useState<PushState>("off");
  const [prefs, setPrefs] = useState<{ off: string[]; kinds: string[]; phones: number } | null>(null);
  const [busy, setBusy] = useState(false);
  const load = () => api<any>("/me/prefs").then(setPrefs).catch(() => {});
  useEffect(() => { pushState().then(setState); load(); }, []);

  const toggleKind = async (k: string) => {
    if (!prefs) return;
    const off = prefs.off.includes(k) ? prefs.off.filter((x) => x !== k) : [...prefs.off, k];
    setPrefs({ ...prefs, off });
    await api("/me/prefs", { method: "PUT", body: { off } });
  };
  const flip = async () => {
    setBusy(true);
    try { setState(await (state === "on" ? disablePush() : enablePush())); await load(); } finally { setBusy(false); }
  };
  const labels: Record<string, string> = { posts: "Tavern posts", needs: "Needs", offers: "Give-aways", runs: "Runs to town", events: "Events", away: "Away notices" };

  return (
    <>
      <h3>🔔 {t("Notifications")}</h3>
      {state === "unsupported" && <p className="small">{t("This browser cannot show notifications. Install the portal to the home screen first (iPhone), or use Chrome.")}</p>}
      {state === "denied" && <p className="small err">{t("Notifications are blocked for this site in the phone's settings.")}</p>}
      {(state === "on" || state === "off") && (
        <p><button className={state === "on" ? "" : "primary"} disabled={busy} onClick={flip}>{state === "on" ? "🔕 " + t("Turn off on this phone") : "🔔 " + t("Enable notifications")}</button>
          {prefs && <span className="small" style={{ marginLeft: ".6rem" }}>{prefs.phones} {t("phones of this house receive them")}</span>}</p>
      )}
      {prefs && (
        <div className="kinds">
          <div className="small">{t("What the house wants to hear about:")}</div>
          {prefs.kinds.map((k) => (
            <label key={k} className="kind"><input type="checkbox" checked={!prefs.off.includes(k)} onChange={() => toggleKind(k)} /> {t(labels[k] || k)}</label>
          ))}
        </div>
      )}
    </>
  );
}
