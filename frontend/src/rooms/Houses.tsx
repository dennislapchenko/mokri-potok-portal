import { useEffect, useState } from "react";
import { api, API, type House, type Me } from "../api";
import { useT } from "../i18n";
import { VillageMap } from "../map/VillageMap";
import { Crest } from "./shared";
import { Notifications } from "./Notifications";
import { AddPhone } from "../AddPhone";

// Houses: everyone sees the list and manages their own house + devices.
// Stewards also create houses, hand out invite links and assign parcels.
export function Houses({ me, houses, refresh, logout }: { me: Me; houses: House[]; refresh: () => Promise<void>; logout: () => void }) {
  const { t } = useT();
  const steward = me.is_steward === 1;
  const [nh, setNh] = useState({ name: "", crest: "🏠", color: "#b5651d", kind: "house" });
  const [nc, setNc] = useState({ name: "", crest: "🌳", color: "#7a8f5a" });
  const [commonOpen, setCommonOpen] = useState(false);
  const realHouses = houses.filter((h) => h.kind !== "common");
  const commons = houses.filter((h) => h.kind === "common");
  const [invites, setInvites] = useState<Record<number, { code?: string; expires_at?: string }>>({});
  const [assign, setAssign] = useState<House | null>(null);
  const [sel, setSel] = useState<string[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [myName, setMyName] = useState(me.name);
  const [myCrest, setMyCrest] = useState(me.crest);
  const [myColor, setMyColor] = useState(me.color);
  const [copied, setCopied] = useState<number | null>(null);
  const [newOpen, setNewOpen] = useState(false);

  useEffect(() => { api<any[]>("/devices").then(setDevices).catch(() => {}); }, []);
  useEffect(() => {
    if (!steward) return;
    houses.filter((h) => h.kind !== "common").forEach((h) => api(`/houses/${h.id}/invite`).then((i) => setInvites((s) => ({ ...s, [h.id]: i }))).catch(() => {}));
  }, [houses, steward]);

  const inviteLink = (code: string) => `${location.origin}${location.pathname}#/join/${code}`;
  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/houses", { method: "POST", body: nh });
    setNh({ name: "", crest: "🏠", color: "#b5651d", kind: "house" }); setNewOpen(false); await refresh();
  };
  const rotate = async (id: number) => { const i = await api(`/houses/${id}/invite`, { method: "POST" }); setInvites((s) => ({ ...s, [id]: i })); };
  const copy = (id: number, code: string) => { navigator.clipboard?.writeText(inviteLink(code)); setCopied(id); setTimeout(() => setCopied(null), 1500); };
  const startAssign = (h: House) => { setAssign(h); setSel([...(h.parcels || [])]); };
  const saveAssign = async () => { if (!assign) return; await api(`/houses/${assign.id}`, { method: "PUT", body: { parcels: sel } }); setAssign(null); await refresh(); };
  const toggleSteward = (h: House) => api(`/houses/${h.id}`, { method: "PUT", body: { is_steward: h.is_steward !== 1 } }).then(refresh);

  return (
    <>
      {assign && (
        <div className="parchment">
          <h2>🗺️ {t("Assign land")}: {assign.crest} {assign.name} <span className="sub">{sel.length} {t("parcels")}</span></h2>
          <p className="small">{t("Tap parcels on the map to toggle, then save.")}</p>
          <VillageMap houses={houses.map((h) => (h.id === assign.id ? { ...h, parcels: [] } : h))} selected={sel} onParcelClick={(p) => setSel((s) => (s.includes(p) ? s.filter((x) => x !== p) : [...s, p]))} />
          <div className="submit" style={{ display: "flex", gap: ".5rem", justifyContent: "flex-end", marginTop: ".6rem" }}>
            <button onClick={() => setAssign(null)}>✕</button>
            <button className="primary" onClick={saveAssign}>{t("Save")}</button>
          </div>
        </div>
      )}
      <div className="parchment">
        <h2>🏘️ {t("Houses")} <span className="sub">{t("houses and land")}</span></h2>
        {realHouses.map((h) => (
          <div key={h.id} className="card" style={{ borderLeftColor: h.color }}>
            <div className="head"><Crest crest={h.crest} color={h.color} /><span className="who">{h.name}</span>{h.kind === "common" && <span className="tag">{t("common land")}</span>}{h.is_steward === 1 && <span className="tag">🗝️ {t("Steward")}</span>}<span className="when">{(h.parcels || []).length} {t("parcels")}{h.parcels?.length ? ": " + h.parcels.join(", ") : ""}</span></div>
            {steward && (
              <div className="actions">
                <button onClick={() => startAssign(h)}>🗺️ {t("Assign land")}</button>
                {invites[h.id]?.code ? (<>
                  <button onClick={() => copy(h.id, invites[h.id].code!)}>🔗 {copied === h.id ? t("Copied") : t("Copy")} {t("Invite link")}</button>
                  <span className="small">{t("valid until")} {invites[h.id].expires_at?.slice(0, 10)}</span>
                </>) : null}
                <button className="ghost" onClick={() => rotate(h.id)}>♻ {t("New link")}</button>
                {h.id !== me.id && <button className="ghost" onClick={() => toggleSteward(h)}>{h.is_steward === 1 ? t("Remove steward") : t("Make steward")}</button>}
                {h.id !== me.id && <button className="ghost" onClick={() => confirm(h.name + "?") && api(`/houses/${h.id}`, { method: "DELETE" }).then(refresh)}>🗑</button>}
              </div>
            )}
            {steward && invites[h.id]?.code && <div className="small" style={{ wordBreak: "break-all" }}>{inviteLink(invites[h.id].code!)}</div>}
          </div>
        ))}
        {steward && (<>
          <p className="small">{t("Send this link to the house in WhatsApp. Everyone in the house opens it once.")}</p>
          <p><button onClick={() => setNewOpen(!newOpen)}>+ {t("New house")}</button></p>
          {newOpen && <form className="inline" onSubmit={create}>
            <div className="row">
              <label>{t("Name")}<input value={nh.name} onChange={(e) => setNh({ ...nh, name: e.target.value })} required maxLength={60} /></label>
              <label>{t("Crest")}<input value={nh.crest} onChange={(e) => setNh({ ...nh, crest: e.target.value })} maxLength={4} /></label>
              <label>{t("Colour")}<input type="color" value={nh.color} onChange={(e) => setNh({ ...nh, color: e.target.value })} /></label>
            </div>
            <div className="submit"><button className="primary" type="submit">{t("Create")}</button></div>
          </form>}
        </>)}
      </div>
      <div className="parchment commons">
        <h2>🌳 {t("Common places")}
          {steward && <button className="lesser" style={{ marginLeft: "auto" }} onClick={() => setCommonOpen(!commonOpen)}>+ {t("Add a place")}</button>}
        </h2>
        {commonOpen && (
          <form className="inline" onSubmit={async (e) => { e.preventDefault(); await api("/houses", { method: "POST", body: { name: nc.name, crest: nc.crest || "🌳", color: nc.color, kind: "common" } }); setNc({ name: "", crest: "🌳", color: "#7a8f5a" }); setCommonOpen(false); await refresh(); }}>
            <div className="row">
              <label>{t("Name")}<input value={nc.name} onChange={(e) => setNc({ ...nc, name: e.target.value })} required maxLength={60} placeholder={t("Event grounds")} /></label>
              <label>{t("Crest")}<input value={nc.crest} onChange={(e) => setNc({ ...nc, crest: e.target.value })} maxLength={4} /></label>
              <label>{t("Colour")}<input type="color" value={nc.color} onChange={(e) => setNc({ ...nc, color: e.target.value })} /></label>
            </div>
            <div className="submit"><button className="primary" type="submit">{t("Create")}</button></div>
          </form>
        )}
        {commons.length === 0 && <p className="small muted">{t("None yet.")}</p>}
        {commons.map((h) => (
          <div key={h.id} className="common-row">
            <Crest crest={h.crest} color={h.color} /> <strong>{h.name}</strong>
            <span className="small">{(h.parcels || []).length} {t("parcels")}</span>
            {steward && <button className="lesser" onClick={() => startAssign(h)}>🗺️ {t("Assign land")}</button>}
            {steward && <button className="ghost" onClick={() => confirm(h.name + "?") && api(`/houses/${h.id}`, { method: "DELETE" }).then(refresh)}>🗑</button>}
          </div>
        ))}
      </div>
      <div className="parchment">
        <h2>{me.crest} {t("Your house")}</h2>
        <form className="inline" onSubmit={(e) => { e.preventDefault(); api(`/houses/${me.id}`, { method: "PUT", body: { name: myName, crest: myCrest, color: myColor } }).then(refresh); }}>
          <div className="row">
            <label>{t("Rename house")}<input value={myName} onChange={(e) => setMyName(e.target.value)} maxLength={60} /></label>
            <label>{t("Crest")}<input value={myCrest} onChange={(e) => setMyCrest(e.target.value)} maxLength={4} /></label>
            <label>{t("Colour")}<input type="color" value={myColor} onChange={(e) => setMyColor(e.target.value)} /></label>
          </div>
          <div className="submit"><button type="submit">{t("Save")}</button></div>
        </form>
        <Notifications steward={steward} />
        <h3>{t("Your devices")}</h3>
        <p><AddPhone /></p>
        {devices.map((d) => (
          <div key={d.id} className="card"><div className="head"><span>📱 {d.label || "—"}{d.id === me.device_id ? ` (${t("This phone")})` : ""}</span><span className="when">{d.last_seen?.slice(0, 16)}</span></div>
            {d.id !== me.device_id && <div className="actions"><button className="ghost" onClick={() => api(`/devices/${d.id}`, { method: "DELETE" }).then(() => api<any[]>("/devices").then(setDevices))}>{t("Remove")}</button></div>}
          </div>
        ))}
        <div className="actions" style={{ marginTop: "1rem", display: "flex", gap: ".5rem", flexWrap: "wrap" }}>
          {steward && <a className="btn" href={`${API}/export`} onClick={async (e) => { e.preventDefault(); const r = await fetch(`${API}/export`, { headers: { Authorization: "Bearer " + (localStorage.getItem("potok.token") || "") } }); const b = await r.blob(); const u = URL.createObjectURL(b); const a = document.createElement("a"); a.href = u; a.download = "potok-export.json"; a.click(); }}><button type="button">📦 {t("Export everything")}</button></a>}
          <button className="danger" onClick={logout}>{t("Log out on this phone")}</button>
        </div>
      </div>
    </>
  );
}
