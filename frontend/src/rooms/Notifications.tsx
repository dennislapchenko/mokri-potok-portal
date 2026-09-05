import { useEffect, useState } from "react";
import { api } from "../api";
import { useT } from "../i18n";
import { disablePush, enablePush, pushState, type PushState } from "../push";

// Notifications: this phone on/off, and the house-wide list of kinds. All kinds
// are on until a house switches one off (the backend stores only the off list).
export function Notifications({ steward }: { steward: boolean }) {
  const { t } = useT();
  const [state, setState] = useState<PushState>("off");
  const [prefs, setPrefs] = useState<{ off: string[]; global_off: string[]; global_detail: { kind: string; set_by: string | null; set_at: string }[]; kinds: string[]; phones: number } | null>(null);
  const mutedBy = (k: string) => { const d = prefs?.global_detail?.find((x) => x.kind === k); return d ? `${t("Switched off for the whole village by")} ${d.set_by || "?"} · ${d.set_at.slice(0, 10)}` : ""; };
  const [busy, setBusy] = useState(false);
  const load = () => api<any>("/me/prefs").then(setPrefs).catch(() => {});
  useEffect(() => { pushState().then(setState); load(); }, []);

  // Steward lever: one switch mutes a kind for every house in the village.
  const toggleGlobal = async (k: string) => {
    if (!prefs) return;
    const off = prefs.global_off.includes(k) ? prefs.global_off.filter((x) => x !== k) : [...prefs.global_off, k];
    setPrefs({ ...prefs, global_off: off });
    await api("/prefs/global", { method: "PUT", body: { off } });
  };
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
  const labels: Record<string, string> = { posts: "Tavern posts", needs: "Needs", offers: "Give-aways", runs: "Runs to town", events: "Events", away: "Away notices", tools: "Tool shed" };

  return (
    <>
      <h3>🔔 {t("Notifications")}</h3>
      {state === "unsupported" && <p className="small">{t("This browser cannot show notifications. Install the portal to the home screen first (iPhone), or use Chrome.")}</p>}
      {state === "denied" && <p className="small err">{t("Notifications are blocked for this site in the phone's settings.")}</p>}
      {(state === "on" || state === "off") && (
        <p><button className={state === "on" ? "" : "primary"} disabled={busy} onClick={flip}>{state === "on" ? "🔕 " + t("Turn off on this phone") : "🔔 " + t("Enable notifications")}</button>
          {prefs && <span className="small" style={{ marginLeft: ".6rem" }}>{prefs.phones} {t("phones of this house receive them")}</span>}</p>
      )}
      <p className="small">{t("Notifications sleep from 21:00 to 07:00 — only an alarm rings at night.")}</p>
      {prefs && (
        <div className="kinds">
          <div className="small">{t("What the house wants to hear about:")}</div>
          {prefs.kinds.map((k) => (
            <label key={k} className={"kind" + (prefs.global_off.includes(k) ? " muted-global" : "")} title={mutedBy(k)}>
              <input type="checkbox" checked={!prefs.off.includes(k)} onChange={() => toggleKind(k)} disabled={prefs.global_off.includes(k)} /> {t(labels[k] || k)}{prefs.global_off.includes(k) ? " 🔇" : ""}
            </label>
          ))}
        </div>
      )}
      {prefs && prefs.global_off.length > 0 && <div className="small">🔇 {prefs.global_detail.map((d) => `${t(labels[d.kind] || d.kind)}: ${d.set_by || "?"} · ${d.set_at.slice(0, 10)}`).join(" · ")}</div>}
      {prefs && steward && (
        <div className="kinds global">
          <div className="small">🗝️ {t("Village-wide (steward): kinds switched off here reach nobody, whatever each house chose. Alarms always ring.")}</div>
          {prefs.kinds.map((k) => (
            <label key={k} className="kind"><input type="checkbox" checked={!prefs.global_off.includes(k)} onChange={() => toggleGlobal(k)} /> {t(labels[k] || k)}</label>
          ))}
        </div>
      )}
    </>
  );
}
