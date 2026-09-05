import { useEffect, useMemo, useRef, useState } from "react";
import { useT } from "./i18n";

// The one date control of the portal, desktop and phone alike: a parchment
// button showing the date in words, opening a month grid in the calendar's own
// style. Values stay plain strings — "YYYY-MM-DD", or "YYYY-MM-DDTHH:MM" when
// `time` is set — so nothing else in the app changes. An empty value opens on
// the current month: the year and month are "prefilled" by where you land.

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

export function DatePicker({ value, onChange, time, placeholder, required, defaultTime = "09:00" }: {
  value: string; onChange: (v: string) => void; time?: boolean; placeholder?: string; required?: boolean; defaultTime?: string;
}) {
  const { t, lang } = useT();
  const locale = lang === "sl" ? "sl-SI" : "en-GB";
  const [open, setOpen] = useState(false);
  const wrap = useRef<HTMLSpanElement>(null);
  const datePart = value.slice(0, 10), timePart = time ? value.slice(11, 16) || defaultTime : "";
  const [cursor, setCursor] = useState(() => (datePart ? new Date(datePart + "T00:00") : new Date()));
  useEffect(() => { if (open) setCursor(datePart ? new Date(datePart + "T00:00") : new Date()); }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => { if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false); };
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", away); document.addEventListener("keydown", esc);
    return () => { document.removeEventListener("mousedown", away); document.removeEventListener("keydown", esc); };
  }, [open]);

  const cells = useMemo(() => {
    const first = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
    const start = new Date(first); start.setDate(1 - ((first.getDay() + 6) % 7));
    return Array.from({ length: 42 }, (_, i) => { const d = new Date(start); d.setDate(start.getDate() + i); return d; });
  }, [cursor]);
  const weekdays = useMemo(() => Array.from({ length: 7 }, (_, i) => new Date(2026, 0, 5 + i).toLocaleDateString(locale, { weekday: "short" }).slice(0, 2)), [locale]);
  const today = iso(new Date());

  const pick = (d: Date) => {
    const day = iso(d);
    onChange(time ? `${day}T${timePart}` : day);
    if (!time) setOpen(false);
  };
  const setTime = (hh: string, mm: string) => onChange(`${datePart || today}T${hh}:${mm}`);

  const shown = datePart
    ? new Date(datePart + "T00:00").toLocaleDateString(locale, { weekday: "short", day: "numeric", month: "short" }) + (time ? ` · ${timePart}` : "")
    : "";
  const [hh, mm] = (timePart || defaultTime).split(":");

  return (
    <span className="dp" ref={wrap}>
      <button type="button" className={"dp-btn" + (shown ? "" : " empty")} onClick={() => setOpen(!open)}>
        📅 {shown || placeholder || t("pick a date")}
      </button>
      {required && <input tabIndex={-1} aria-hidden="true" required value={value} onChange={() => {}} className="dp-req" />}
      {open && (
        <div className="dp-pop" role="dialog">
          <div className="cal-head">
            <button type="button" className="ghost" onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() - 1, 1))}>‹</button>
            <strong>{cursor.toLocaleDateString(locale, { month: "long", year: "numeric" })}</strong>
            <button type="button" className="ghost" onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1))}>›</button>
          </div>
          <div className="cal-grid dp-grid">
            {weekdays.map((w, i) => <div key={"w" + i} className="cal-wd">{w}</div>)}
            {cells.map((d) => {
              const k = iso(d);
              return (
                <button type="button" key={k} className={"cal-day" + (d.getMonth() !== cursor.getMonth() ? " other" : "") + (k === today ? " today" : "") + (k === datePart ? " picked" : "")} onClick={() => pick(d)}>
                  <span className="n">{d.getDate()}</span>
                </button>
              );
            })}
          </div>
          {time && (
            <div className="dp-time">
              🕰️
              <select value={hh} onChange={(e) => setTime(e.target.value, mm)}>{Array.from({ length: 24 }, (_, i) => String(i).padStart(2, "0")).map((h) => <option key={h} value={h}>{h}</option>)}</select>
              :
              <select value={mm} onChange={(e) => setTime(hh, e.target.value)}>{["00", "15", "30", "45"].map((m) => <option key={m} value={m}>{m}</option>)}</select>
            </div>
          )}
          <div className="dp-foot">
            <button type="button" className="ghost" onClick={() => { onChange(""); setOpen(false); }}>{t("Clear")}</button>
            <button type="button" className="lesser" onClick={() => pick(new Date())}>{t("today")}</button>
            <button type="button" className="primary" onClick={() => setOpen(false)}>{t("Done")}</button>
          </div>
        </div>
      )}
    </span>
  );
}
