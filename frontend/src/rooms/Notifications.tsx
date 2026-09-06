import { useEffect, useState } from "react";
import { api } from "../api";
import { useT } from "../i18n";
import { disablePush, enablePush, pushState, type PushState } from "../push";

// Notifications: this phone on/off, and the house-wide list of kinds. All kinds
// are on until a house switches one off (the backend stores only the off list).
export function Notifications({ steward }: { steward: boolean }) {
  const { t } = useT();
  const [state, setState] = useState<PushState>("off");
  const [prefs, setPrefs] = useState<{ off: string[]; global_off: string[]; global_detail: { kind: string; set_by: string | null; set_at: string }[]; kinds: string[]; phones: number; quiet_ok: boolean } | null>(null);
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
  // Quiet hours are the only device-scoped preference: the phone that rings at
  // 23:00 belongs to a person, not to the house.
  const toggleQuiet = async () => {
    if (!prefs) return;
    const quiet_ok = !prefs.quiet_ok;
    setPrefs({ ...prefs, quiet_ok });
    await api("/me/device", { method: "PUT", body: { quiet_ok } });
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
  const labels: Record<string, string> = { posts: "Tavern posts", needs: "Needs", offers: "Give-aways", runs: "Runs to town", events: "Events", away: "Away notices", tools: "Tool shed", projects: "Projects", camp: "Campground" };

  return (
    <>
      <h3>🔔 {t("Notifications")}</h3>
      {state === "unsupported" && <p className="small">{t("This browser cannot show notifications. Install the portal to the home screen first (iPhone), or use Chrome.")}</p>}
      {state === "denied" && <p className="small err">{t("Notifications are blocked for this site in this device's settings.")}</p>}
      {(state === "on" || state === "off") && (
        <p><button className={state === "on" ? "" : "primary"} disabled={busy} onClick={flip}>{state === "on" ? "🔕 " + t("Turn off on this device") : "🔔 " + t("Enable notifications")}</button>
          {prefs && <span className="small" style={{ marginLeft: ".6rem" }}>{prefs.phones} {t("devices of this house receive them")}</span>}</p>
      )}
      {prefs && state !== "unsupported" && (
        <div className="kinds">
          <div className="small">🌙 {t("Notifications sleep from 21:00 to 07:00. Nothing rings at night unless a device asks for it.")}</div>
          <label className="kind wide"><input type="checkbox" checked={prefs.quiet_ok} onChange={toggleQuiet} /> {t("Ring at night on this device too")}</label>
          <div className="small muted">{t("This device only. The other devices of the house keep sleeping.")}</div>
        </div>
      )}
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
          <div className="small">🗝️ {t("Village-wide (steward): kinds switched off here reach nobody, whatever each house chose.")}</div>
          {prefs.kinds.map((k) => (
            <label key={k} className="kind"><input type="checkbox" checked={!prefs.global_off.includes(k)} onChange={() => toggleGlobal(k)} /> {t(labels[k] || k)}</label>
          ))}
        </div>
      )}
    </>
  );
}
