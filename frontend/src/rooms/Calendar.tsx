import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, type House, type Me } from "../api";
import { useT } from "../i18n";
import { useList } from "./shared";
import { EventCard, ICON } from "./EventCard";
import { DatePicker } from "../DatePicker";

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

// The village calendar: a month grid with the events sitting in their day, then
// the chosen day (or everything ahead) as cards you can sign up to.
export function Calendar({ me, houses }: { me: Me; houses: House[] }) {
  const { t, lang } = useT();
  const { items, reload } = useList("/events");
  const [f, setF] = useState({ title: "", kind: "event", starts_at: "", ends_at: "", place: "", notes: "", project_id: "", task_id: "" });
  const [open, setOpen] = useState(false);
  // Projects for the "belongs to" select; ?project=ID (from a project page)
  // opens the form with that project chosen.
  const projects = useList("/projects");
  const [params] = useSearchParams();
  const [tasks, setTasks] = useState<any[]>([]);
  useEffect(() => {
    if (!f.project_id) { setTasks([]); return; }
    api<any>(`/projects/${f.project_id}`).then((p) => setTasks((p.tasks || []).filter((x: any) => x.state === "open"))).catch(() => setTasks([]));
  }, [f.project_id]);
  useEffect(() => {
    const pid = params.get("project");
    if (pid && !open) { const d = defaultDay(); setF((x) => ({ ...x, project_id: pid, kind: "work", starts_at: d + "T09:00", ends_at: d + "T17:00" })); setOpen(true); }
    const dd = params.get("day");
    if (dd && /^\d{4}-\d{2}-\d{2}$/.test(dd)) { setDay(dd); setCursor(new Date(dd + "T00:00")); }
  }, [params]); // eslint-disable-line react-hooks/exhaustive-deps
  const [cursor, setCursor] = useState(() => new Date());
  const [day, setDay] = useState<string | null>(null);
  const locale = lang === "sl" ? "sl-SI" : "en-GB";

  // The form opens already filled: the picked day, else today when the shown
  // month is this one, else the 1st of the shown month — 09:00 to 17:00. A
  // villager then changes the day, or nothing. Moving the start date drags the
  // end date along until the end is edited by hand.
  const [endTouched, setEndTouched] = useState(false);
  const defaultDay = () => {
    if (day) return day;
    const now = new Date();
    return cursor.getFullYear() === now.getFullYear() && cursor.getMonth() === now.getMonth() ? iso(now) : iso(new Date(cursor.getFullYear(), cursor.getMonth(), 1));
  };
  const openForm = () => {
    if (!open) {
      const d = defaultDay();
      setF({ ...f, starts_at: d + "T09:00", ends_at: d + "T17:00" });
      setEndTouched(false);
    }
    setOpen(!open);
  };
  const set = (k: string) => (e: React.ChangeEvent<any>) => {
    const v = e.target.value;
    if (k === "starts_at" && !endTouched && v.length >= 10) {
      setF({ ...f, starts_at: v, ends_at: v.slice(0, 10) + (f.ends_at.slice(10) || "T17:00") });
      return;
    }
    if (k === "ends_at") setEndTouched(true);
    setF({ ...f, [k]: v });
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/events", { method: "POST", body: { ...f, project_id: f.project_id ? Number(f.project_id) : undefined, task_id: f.task_id ? Number(f.task_id) : undefined } });
    setF({ title: "", kind: "event", starts_at: "", ends_at: "", place: "", notes: "", project_id: "", task_id: "" });
    setOpen(false);
    reload();
  };

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

  return (
    <div className="parchment">
      <h2>🔔 {t("The village calendar")} <span className="sub">{t("calendar and work bees")}</span>
        <button style={{ marginLeft: "auto" }} onClick={openForm}>+ {t("Add an event")}</button>
      </h2>

      {open && (
        <form className="inline" onSubmit={save}>
          <div className="row">
            <label>{t("Title")}<input value={f.title} onChange={set("title")} required maxLength={120} /></label>
            <label>{t("Kind")}<select value={f.kind} onChange={set("kind")}><option value="event">🔔 {t("event")}</option><option value="work">🤝 {t("work")}</option><option value="alarm">🚨 {t("alarm")}</option></select></label>
          </div>
          <div className="row">
            <label>{t("Starts")}<DatePicker time required value={f.starts_at} onChange={(v) => set("starts_at")({ target: { value: v } } as any)} /></label>
            <label>{t("Ends")}<DatePicker time value={f.ends_at} onChange={(v) => set("ends_at")({ target: { value: v } } as any)} defaultTime="17:00" /></label>
            <label>{t("Place")}<span className="place-pick">
              <input value={f.place} onChange={set("place")} maxLength={120} placeholder={t("anywhere, or pick below")} />
              <select value="" onChange={(e) => e.target.value && setF({ ...f, place: e.target.value })}>
                <option value="">📍…</option>
                {houses.map((h) => <option key={h.id} value={h.name}>{h.crest} {h.name}</option>)}
              </select>
            </span></label>
          </div>
          {projects.items.filter((p) => p.state === "open").length > 0 && (
            <div className="row">
              <label>📋 {t("Belongs to a project")}<select value={f.project_id} onChange={(e) => setF({ ...f, project_id: e.target.value, task_id: "" })}><option value="">—</option>{projects.items.filter((p) => p.state === "open").map((p) => <option key={p.id} value={p.id}>{p.title}</option>)}</select></label>
              {f.project_id && tasks.length > 0 && <label>{t("Task")}<select value={f.task_id} onChange={set("task_id")}><option value="">—</option>{tasks.map((x) => <option key={x.id} value={x.id}>{x.title}</option>)}</select></label>}
            </div>
          )}
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
                <span className={"marks m" + Math.min(evs.length, 4)}>{evs.slice(0, 4).map((e, i) => <span key={i}>{ICON[e.kind]}</span>)}{evs.length > 4 ? <span className="more">+{evs.length - 4}</span> : null}</span>
              </button>
            );
          })}
        </div>
      </div>

      {day && <p className="small">{new Date(day + "T00:00").toLocaleDateString(locale, { weekday: "long", day: "numeric", month: "long" })} · <button className="ghost" onClick={() => setDay(null)}>{t("Show the whole month")}</button></p>}
      {shown.length === 0 && <p className="muted" style={{ fontStyle: "italic" }}>{day ? t("Nothing on this day.") : t("Nothing planned. Ring the bell — add an event or call a work party, and houses sign up here.")}</p>}
      {shown.map((ev) => <EventCard key={ev.id + "-" + (day || "")} ev={ev} me={me} reload={reload} />)}
    </div>
  );
}
