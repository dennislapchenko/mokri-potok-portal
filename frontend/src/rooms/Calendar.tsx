import { useMemo, useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, When, canEdit, useList } from "./shared";

const ICON: Record<string, string> = { event: "🔔", work: "🤝", alarm: "🚨" };
const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

// The village calendar: a month grid with the events sitting in their day, then
// the chosen day (or everything ahead) as cards you can sign up to.
export function Calendar({ me }: { me: Me }) {
  const { t, lang } = useT();
  const { items, reload } = useList("/events");
  const [f, setF] = useState({ title: "", kind: "event", starts_at: "", ends_at: "", place: "", notes: "" });
  const [open, setOpen] = useState(false);
  const [cursor, setCursor] = useState(() => new Date());
  const [day, setDay] = useState<string | null>(null);
  const set = (k: string) => (e: React.ChangeEvent<any>) => setF({ ...f, [k]: e.target.value });
  const locale = lang === "sl" ? "sl-SI" : "en-GB";

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/events", { method: "POST", body: f });
    setF({ title: "", kind: "event", starts_at: "", ends_at: "", place: "", notes: "" });
    setOpen(false);
    reload();
  };
  const signup = (ev: any) => api(`/events/${ev.id}/signup`, { method: ev.mine ? "DELETE" : "POST" }).then(reload);

  // An event occupies every day from its start to its end.
  const byDay = useMemo(() => {
    const m: Record<string, any[]> = {};
    for (const ev of items) {
      const from = new Date(ev.starts_at.slice(0, 10) + "T00:00");
      const to = new Date((ev.ends_at || ev.starts_at).slice(0, 10) + "T00:00");
      for (const d = new Date(from); d <= to; d.setDate(d.getDate() + 1)) (m[iso(d)] ||= []).push(ev);
    }
    return m;
  }, [items]);

  // Monday-first grid, six rows so the layout never jumps between months.
  const cells = useMemo(() => {
    const first = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
    const start = new Date(first);
    start.setDate(1 - ((first.getDay() + 6) % 7));
    return Array.from({ length: 42 }, (_, i) => {
      const d = new Date(start);
      d.setDate(start.getDate() + i);
      return d;
    });
  }, [cursor]);

  const today = iso(new Date());
  const shown = day ? byDay[day] || [] : items.filter((ev) => (ev.ends_at || ev.starts_at).slice(0, 10) >= today);
  const weekdays = useMemo(() => {
    const base = new Date(2026, 0, 5); // a Monday
    return Array.from({ length: 7 }, (_, i) => {
      const d = new Date(base);
      d.setDate(5 + i);
      return d.toLocaleDateString(locale, { weekday: "short" }).slice(0, 2);
    });
  }, [locale]);

  const Card = ({ ev }: { ev: any }) => (
    <div className="card" style={{ borderLeftColor: ev.kind === "alarm" ? "var(--red)" : ev.kind === "work" ? "var(--green)" : "var(--brass)" }}>
      <div className="head">
        <Crest crest={ev.house_crest} color={ev.house_color} />
        <strong>{ICON[ev.kind]} {ev.title}</strong>
        {ev.kind !== "event" && <span className={"tag " + ev.kind}>{t(ev.kind)}</span>}
        <span className="when"><When iso={ev.starts_at} />{ev.ends_at ? <> → <When iso={ev.ends_at} /></> : null}</span>
      </div>
      {(ev.place || ev.notes) && <div className="body small">{ev.place && <>📍 {ev.place} </>}{ev.notes}</div>}
      <div className="small">🙋 {ev.signups > 0 ? ev.signup_names : <span className="muted">{t("nobody has signed up yet")}</span>}</div>
      <div className="actions">
        <button className={ev.mine ? "" : "primary"} onClick={() => signup(ev)}>
          {ev.mine ? t("I cannot come after all") : "🙋 " + t("I am coming")}{ev.signups > 0 ? ` (${ev.signups})` : ""}
        </button>
        {canEdit(me, ev) && <button className="ghost" onClick={() => confirm("?") && api(`/events/${ev.id}`, { method: "DELETE" }).then(reload)}>🗑 {t("Delete")}</button>}
      </div>
    </div>
  );

  return (
    <div className="parchment">
      <h2>🔔 {t("The village calendar")} <span className="sub">{t("calendar and work bees")}</span>
        <button style={{ marginLeft: "auto" }} onClick={() => setOpen(!open)}>+ {t("Add an event")}</button>
      </h2>

      {open && (
        <form className="inline" onSubmit={save}>
          <div className="row">
            <label>{t("Title")}<input value={f.title} onChange={set("title")} required maxLength={120} /></label>
            <label>{t("Kind")}<select value={f.kind} onChange={set("kind")}><option value="event">🔔 {t("event")}</option><option value="work">🤝 {t("work")}</option><option value="alarm">🚨 {t("alarm")}</option></select></label>
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

      <div className="cal">
        <div className="cal-head">
          <button className="ghost" aria-label="‹" onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() - 1, 1))}>‹</button>
          <strong>{cursor.toLocaleDateString(locale, { month: "long", year: "numeric" })}</strong>
          <button className="ghost" aria-label="›" onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1))}>›</button>
        </div>
        <div className="cal-grid">
          {weekdays.map((w, i) => <div key={"w" + i} className="cal-wd">{w}</div>)}
          {cells.map((d) => {
            const key = iso(d);
            const evs = byDay[key] || [];
            const other = d.getMonth() !== cursor.getMonth();
            return (
              <button key={key} className={"cal-day" + (other ? " other" : "") + (key === today ? " today" : "") + (key === day ? " picked" : "")}
                onClick={() => setDay(key === day ? null : key)}>
                <span className="n">{d.getDate()}</span>
                <span className="marks">{evs.slice(0, 3).map((e, i) => <span key={i}>{ICON[e.kind]}</span>)}{evs.length > 3 ? "…" : ""}</span>
              </button>
            );
          })}
        </div>
      </div>

      {day && <p className="small">{new Date(day + "T00:00").toLocaleDateString(locale, { weekday: "long", day: "numeric", month: "long" })} · <button className="ghost" onClick={() => setDay(null)}>{t("Show the whole month")}</button></p>}
      {shown.length === 0 && <p className="muted" style={{ fontStyle: "italic" }}>{day ? t("Nothing on this day.") : t("Nothing planned. Ring the bell — add an event or call a work party, and houses sign up here.")}</p>}
      {shown.map((ev) => <Card key={ev.id + "-" + (day || "")} ev={ev} />)}
    </div>
  );
}
