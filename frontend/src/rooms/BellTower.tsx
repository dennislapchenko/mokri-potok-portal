import { useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, canEdit, useList } from "./shared";

export function BellTower({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/events");
  const [f, setF] = useState({ title: "", kind: "event", starts_at: "", ends_at: "", place: "", notes: "" });
  const [open, setOpen] = useState(false);
  const set = (k: string) => (e: React.ChangeEvent<any>) => setF({ ...f, [k]: e.target.value });

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/events", { method: "POST", body: f });
    setF({ title: "", kind: "event", starts_at: "", ends_at: "", place: "", notes: "" }); setOpen(false); reload();
  };
  const today = new Date().toISOString().slice(0, 10);
  const upcoming = items.filter((x) => (x.ends_at || x.starts_at) >= today);
  const past = items.filter((x) => (x.ends_at || x.starts_at) < today).reverse();
  const byMonth: Record<string, any[]> = {};
  for (const ev of upcoming) (byMonth[ev.starts_at.slice(0, 7)] ||= []).push(ev);

  const Card = ({ ev }: { ev: any }) => (
    <div className="card" style={{ borderLeftColor: ev.kind === "alarm" ? "var(--red)" : ev.kind === "work" ? "var(--green)" : "var(--brass)" }}>
      <div className="head"><Crest crest={ev.house_crest} color={ev.house_color} /><strong>{ev.title}</strong>{ev.kind !== "event" && <span className={"tag " + ev.kind}>{t(ev.kind)}</span>}<span className="when"><When iso={ev.starts_at} />{ev.ends_at ? <> → <When iso={ev.ends_at} /></> : null}</span></div>
      {(ev.place || ev.notes) && <div className="body small">{ev.place && <>📍 {ev.place} </>}{ev.notes}</div>}
      {canEdit(me, ev) && <div className="actions"><button className="ghost" onClick={() => confirm("?") && api(`/events/${ev.id}`, { method: "DELETE" }).then(reload)}>🗑 {t("Delete")}</button></div>}
    </div>
  );

  return (
    <div className="parchment">
      <h2>🔔 {t("Bell tower")} <span className="sub">{t("calendar")}</span><button style={{ marginLeft: "auto" }} onClick={() => setOpen(!open)}>+ {t("Add an event")}</button></h2>
      {open && (
        <form className="inline" onSubmit={save}>
          <div className="row">
            <label>{t("Title")}<input value={f.title} onChange={set("title")} required maxLength={120} /></label>
            <label>{t("Kind")}<select value={f.kind} onChange={set("kind")}><option value="event">{t("event")}</option><option value="work">{t("work")}</option><option value="alarm">{t("alarm")}</option></select></label>
          </div>
          <div className="row">
            <label>{t("Starts")}<input type="datetime-local" value={f.starts_at} onChange={set("starts_at")} required /></label>
            <label>{t("Ends")}<input type="datetime-local" value={f.ends_at} onChange={set("ends_at")} /></label>
            <label>{t("Place")}<input value={f.place} onChange={set("place")} maxLength={120} /></label>
          </div>
          <label>{t("Notes")}<textarea value={f.notes} onChange={set("notes")} maxLength={2000} /></label>
          <div className="submit"><button className="primary" type="submit">{t("Save")}</button></div>
        </form>
      )}
      {upcoming.length === 0 && <Empty text={t("Nothing planned. Ring the bell.")} />}
      {Object.entries(byMonth).map(([m, evs]) => (
        <div key={m}>
          <h3 style={{ marginTop: ".8rem", color: "var(--ink2)" }}>{new Date(m + "-01").toLocaleString(undefined, { month: "long", year: "numeric" })}</h3>
          {evs.map((ev) => <Card key={ev.id} ev={ev} />)}
        </div>
      ))}
      {past.length > 0 && <details style={{ marginTop: "1rem" }}><summary className="small">{past.length} ⏳</summary>{past.map((ev) => <Card key={ev.id} ev={ev} />)}</details>}
    </div>
  );
}
