import { useState } from "react";
import { Link } from "react-router-dom";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, When, canEdit, isOver } from "./shared";
import { Thread } from "./Thread";
import { DatePicker } from "../DatePicker";

// `alarm` is gone (2026-09-06) but old rows still render.
export const ICON: Record<string, string> = { event: "🔔", work: "🤝", alarm: "🚨" };
const RSVP: { state: string; icon: string; label: string }[] = [
  { state: "yes", icon: "🙋", label: "I am coming" },
  { state: "no", icon: "❌", label: "I cannot come" },
  { state: "maybe", icon: "🤔", label: "Maybe" },
];

// One event, wherever it is shown. Three answers, never blended: a house that
// says no is not the same as a house that has not answered, and only "coming"
// is counted. A sign-up carries no note — "I bring the scythe" belongs in the
// comment thread below, which every house reads and can answer.
export function EventCard({ ev, me, reload, linkToTavern }: { ev: any; me: Me; reload: () => void; linkToTavern?: boolean }) {
  const { t } = useT();
  const [editing, setEditing] = useState(false);
  const [f, setF] = useState({ title: ev.title, kind: ev.kind, starts_at: ev.starts_at, ends_at: ev.ends_at || "", place: ev.place || "", notes: ev.notes || "" });
  const [showComments, setShowComments] = useState(false);

  const list = JSON.parse(ev.signup_list || "[]") as any[];
  const mine = ev.mine as string | null;
  const by = (s: string) => list.filter((x) => x.state === s);
  const answer = (state: string) => api(`/events/${ev.id}/signup`, { method: "POST", body: { state } }).then(reload);
  const clear = () => api(`/events/${ev.id}/signup`, { method: "DELETE" }).then(reload);
  const save = async () => { await api(`/events/${ev.id}`, { method: "PUT", body: f }); setEditing(false); reload(); };

  const anyStale = list.some((x) => x.stale);
  // Faded, kind colour off the edge, no answer buttons — the thread and the
  // answers given stay. CLAUDE.md § An answer is not a headcount says why.
  const over = isOver(ev);
  // A render function, not a component — see Market.tsx. Lowercase on purpose.
  const group = ({ state, icon }: { state: string; icon: string }) => {
    const g = by(state);
    if (!g.length) return null;
    return (
      <div key={state} className="small signers">
        <span className="state" title={t(RSVP.find((r) => r.state === state)!.label)}>{icon}</span>
        <span className="who">{g.map((sgn, i) => (
          <span key={sgn.house_id} className={sgn.stale ? "stale" : ""}>
            {i > 0 ? ", " : ""}{sgn.crest} {sgn.name}
          </span>
        ))}</span>
      </div>
    );
  };

  return (
    <div className={"card" + (over ? " faded" : "")} style={{ borderLeftColor: over ? "var(--parch3)" : ev.kind === "alarm" ? "var(--red)" : ev.kind === "work" ? "var(--green)" : "var(--brass)" }}>
      {editing ? (
        <form className="inline" onSubmit={(e) => { e.preventDefault(); save(); }}>
          <div className="row">
            <label>{t("Title")}<input value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} required maxLength={120} /></label>
            <label>{t("Kind")}<select value={f.kind} onChange={(e) => setF({ ...f, kind: e.target.value })}>{["event", "work"].map((k) => <option key={k} value={k}>{ICON[k]} {t(k)}</option>)}</select></label>
          </div>
          <div className="row">
            <label>{t("Starts")}<DatePicker time required value={f.starts_at} onChange={(v) => setF({ ...f, starts_at: v, ends_at: f.ends_at && f.ends_at < v ? v : f.ends_at })} /></label>
            <label>{t("Ends")}<DatePicker time min={f.starts_at} value={f.ends_at} onChange={(v) => setF({ ...f, ends_at: v })} defaultTime="17:00" /></label>
            <label>{t("Place")}<input value={f.place} onChange={(e) => setF({ ...f, place: e.target.value })} maxLength={120} /></label>
          </div>
          <label>{t("Notes")}<textarea value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} maxLength={2000} /></label>
          <div className="submit"><button type="button" className="ghost" onClick={() => setEditing(false)}>✕</button><button className="primary" type="submit">{t("Save")}</button></div>
        </form>
      ) : (
        <>
          <div className="head">
            <Crest crest={ev.house_crest} color={ev.house_color} />
            <strong>{ICON[ev.kind]} {linkToTavern ? <Link to={`/tavern?day=${ev.starts_at.slice(0, 10)}`} className="plain">{ev.title}</Link> : ev.title}</strong>
            {ev.kind !== "event" && <span className={"tag " + ev.kind}>{t(ev.kind)}</span>}
            {ev.project_id && <Link to={`/projects/${ev.project_id}`} className="tag project-chip">📋 {ev.project_title}{ev.task_title ? ` · ${ev.task_title}` : ""}</Link>}
            <span className="when"><When iso={ev.starts_at} />{ev.ends_at ? <> → <When iso={ev.ends_at} /></> : null}</span>
          </div>
          {(ev.place || ev.notes) && <div className="body small">{ev.place && <span className="tag place">📍 {ev.place}</span>}{ev.place && ev.notes ? " " : ""}{ev.notes}</div>}
          {ev.edited_by_name && <div className="small muted">✎ {t("last edited by")} {ev.edited_by_name}</div>}
          {anyStale && <div className="small stale-note">⧗ {t("The time moved. Struck answers were given for the old one.")}</div>}
        </>
      )}

      {RSVP.map(group)}
      <div className="actions">
        {!over && RSVP.map((r) => (
          <button key={r.state} className={mine === r.state ? "primary" : "lesser"} onClick={() => (mine === r.state ? clear() : answer(r.state))} title={mine === r.state ? t("tap again to take it back") : ""}>
            {r.icon} {t(r.label)}{r.state === "yes" && ev.signups > 0 ? ` (${ev.signups})` : ""}
          </button>
        ))}
        <button className="ghost" onClick={() => setShowComments(!showComments)}>💬 {t("Comments")}{ev.comments ? ` (${ev.comments})` : ""}</button>
        {!editing && <button className="ghost" onClick={() => setEditing(true)}>✎ {t("Edit")}</button>}
        {canEdit(me, ev) && <button className="ghost" onClick={() => confirm(ev.title + "?") && api(`/events/${ev.id}`, { method: "DELETE" }).then(reload)}>🗑 {t("Delete")}</button>}
      </div>
      {showComments && <Thread subject="event" id={ev.id} me={me} onChanged={reload} />}
    </div>
  );
}
