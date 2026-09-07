import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useT } from "./i18n";

// The one date control of the portal, desktop and phone alike: a parchment
// button showing the date in words, opening a month grid in the calendar's own
// style. Values stay plain strings — "YYYY-MM-DD", or "YYYY-MM-DDTHH:MM" when
// `time` is set — so nothing else in the app changes. An empty value opens on
// the current month: the year and month are "prefilled" by where you land.
//
// `min` makes this picker the far end of a range: the earliest value it may
// take, in the same shape as `value`. Everything before it is a dead cell — a
// disabled day, a disabled hour — because a picker that silently moves the day
// you pressed is worse than one that shows you the door is shut. It is the
// only guard the form needs; the server refuses a backwards range anyway.

// Kept in step with the width in `.dp-pop`: the popup is measured before it
// has been laid out, to place it without a visible jump.
const POP_W = (): number => Math.min(320, window.innerWidth - 24);

const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

export function DatePicker({ value, onChange, time, placeholder, required, defaultTime = "09:00", min }: {
  value: string; onChange: (v: string) => void; time?: boolean; placeholder?: string; required?: boolean; defaultTime?: string; min?: string;
}) {
  const { t, lang } = useT();
  const locale = lang === "sl" ? "sl-SI" : "en-GB";
  const [open, setOpen] = useState(false);
  // The right-hand field of a two-column row has no room for a popup hanging
  // off its left edge — on a narrow phone it ran past the screen, which made
  // the whole page pan sideways and swallowed taps as scrolls. Anchor it to
  // the field's right edge instead.
  const [flip, setFlip] = useState(false);
  const wrap = useRef<HTMLSpanElement>(null);
  const datePart = value.slice(0, 10), timePart = time ? value.slice(11, 16) || defaultTime : "";
  const [cursor, setCursor] = useState(() => (datePart ? new Date(datePart + "T00:00") : new Date()));
  useEffect(() => { if (open) setCursor(datePart ? new Date(datePart + "T00:00") : new Date()); }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useLayoutEffect(() => {
    if (!open || !wrap.current) return;
    setFlip(wrap.current.getBoundingClientRect().left + POP_W() > window.innerWidth - 8);
  }, [open]);

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
  const minDay = min ? min.slice(0, 10) : "";
  const early = (v: string) => !!min && v < min;

  const pick = (d: Date) => {
    const day = iso(d);
    const v = time ? `${day}T${timePart}` : day;
    // Only reachable on the boundary day itself, whose cell stays open: the
    // day is allowed, the hour still standing in the field is not.
    onChange(early(v) ? min! : v);
    if (!time) setOpen(false);
  };
  const setTime = (hh: string, mm: string) => onChange(`${datePart || today}T${hh}:${mm}`);
  // Every room puts this control inside a <label>, and a label forwards a click
  // on anything that is not its own control to `.dp-btn` — which toggles the
  // popup straight back open, so picking a day or pressing Done looked like
  // nothing happening at all. That forward is the click's default action;
  // cancelling it leaves the popup buttons' own handlers untouched.
  const swallow = (e: React.MouseEvent) => e.preventDefault();

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
        <div className={"dp-pop" + (flip ? " flip" : "")} role="dialog" onClick={swallow}>
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
                <button type="button" key={k} disabled={k < minDay} className={"cal-day" + (d.getMonth() !== cursor.getMonth() ? " other" : "") + (k === today ? " today" : "") + (k === datePart ? " picked" : "")} onClick={() => pick(d)}>
                  <span className="n">{d.getDate()}</span>
                </button>
              );
            })}
          </div>
          {time && (
            <div className="dp-time">
              🕰️
              <select value={hh} onChange={(e) => setTime(e.target.value, mm)}>{Array.from({ length: 24 }, (_, i) => String(i).padStart(2, "0")).map((h) => <option key={h} value={h} disabled={early(`${datePart || today}T${h}:${mm}`)}>{h}</option>)}</select>
              :
              <select value={mm} onChange={(e) => setTime(hh, e.target.value)}>{["00", "15", "30", "45"].map((m) => <option key={m} value={m} disabled={early(`${datePart || today}T${hh}:${m}`)}>{m}</option>)}</select>
            </div>
          )}
          <div className="dp-foot">
            <button type="button" className="ghost" onClick={() => { onChange(""); setOpen(false); }}>{t("Clear")}</button>
            <button type="button" className="lesser" disabled={today < minDay} onClick={() => pick(new Date())}>{t("today")}</button>
            <button type="button" className="primary" onClick={() => setOpen(false)}>{t("Done")}</button>
          </div>
        </div>
      )}
    </span>
  );
}
