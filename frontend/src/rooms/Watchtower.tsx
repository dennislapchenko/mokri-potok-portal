import { useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, canEdit, useList } from "./shared";
import { DatePicker } from "../DatePicker";

export function Watchtower({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/away");
  const [f, setF] = useState({ from_date: "", to_date: "", notes: "" });
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/away", { method: "POST", body: f });
    setF({ from_date: "", to_date: "", notes: "" }); reload();
  };
  const today = new Date().toISOString().slice(0, 10);
  return (
    <div className="parchment">
      <h2>🕯️ {t("Watchtower")} <span className="sub">{t("who is away")}</span></h2>
      <p className="small">{t("Only logged-in houses see this room.")}</p>
      <form className="inline" onSubmit={save}>
        <div className="row">
          <label>{t("From")}<DatePicker required value={f.from_date} onChange={(v) => setF({ ...f, from_date: v, to_date: f.to_date && f.to_date < v ? v : f.to_date })} /></label>
          <label>{t("To")}<DatePicker required value={f.to_date} onChange={(v) => setF({ ...f, to_date: v })} /></label>
        </div>
        <label>{t("Notes")}<textarea value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} placeholder={t("What needs care (animals, watering, greenhouse)")} maxLength={2000} /></label>
        <div className="submit"><button className="primary" type="submit">🧳 {t("Mark us away")}</button></div>
      </form>
      {items.length === 0 && <Empty text={t("Nobody is away. All lanterns lit.")} />}
      {items.map((a) => {
        const now = a.from_date <= today && a.to_date >= today;
        return (
          <div key={a.id} className="card" style={{ borderLeftColor: now ? "var(--red)" : "var(--brass)" }}>
            <div className="head"><Crest crest={a.house_crest} color={a.house_color} /><span className="who">{a.house_name}</span>{now && <span className="tag alarm">{t("away now")}</span>}<span className="when"><When iso={a.from_date} /> → <When iso={a.to_date} /></span></div>
            {a.house_about && <div className="small muted">{a.house_about}</div>}
            {a.notes && <div className="body">{a.notes}</div>}
            <div className="small">{a.watcher ? <>👁 {t("Watched by")}: <strong>{a.watcher_name}</strong></> : <span className="muted">—</span>}</div>
            <div className="actions">
              {!a.watcher && a.house_id !== me.id && <button className="primary" onClick={() => api(`/away/${a.id}`, { method: "PUT", body: { watch: true } }).then(reload)}>👁 {t("I will watch")}</button>}
              {a.watcher === me.id && <button onClick={() => api(`/away/${a.id}`, { method: "PUT", body: { watch: false } }).then(reload)}>{t("Step back")}</button>}
              {canEdit(me, a) && <button className="ghost" onClick={() => confirm("?") && api(`/away/${a.id}`, { method: "DELETE" }).then(reload)}>🗑 {t("Delete")}</button>}
            </div>
          </div>
        );
      })}
    </div>
  );
}
