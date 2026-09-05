import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api";
import { useT } from "../i18n";

// useList: load a resource, expose reload. Rooms are lists with a form on top.
export function useList<T = any>(path: string) {
  const [items, setItems] = useState<T[]>([]);
  const [err, setErr] = useState("");
  const reload = useCallback(() => api<T[]>(path).then(setItems).catch((e) => setErr(String(e.message))), [path]);
  useEffect(() => { reload(); }, [reload]);
  return { items, err, reload };
}

export function Crest({ crest, color }: { crest: string; color: string }) {
  return <span className="crest" style={{ background: color }}>{crest}</span>;
}

export function When({ iso }: { iso?: string | null }) {
  const { lang } = useT();
  if (!iso) return null;
  const d = new Date(iso.length <= 10 ? iso + "T00:00" : iso);
  const opts: Intl.DateTimeFormatOptions = iso.length <= 10 ? { day: "numeric", month: "short" } : { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" };
  return <time dateTime={iso}>{d.toLocaleString(lang === "sl" ? "sl-SI" : "en-GB", opts)}</time>;
}

export function Empty({ text }: { text: string }) {
  return <p className="muted" style={{ fontStyle: "italic" }}>{text}</p>;
}

export const canEdit = (me: { id: number; is_steward: number }, row: { house_id: number }) => row.house_id === me.id || me.is_steward === 1;

// DueInput: a date typed as YYYY-MM-DD with the year already there, because a
// native date field cannot hold a year alone. The 📅 opens the native picker.
// An unfinished value ("2026-") counts as no date.
export const yearPrefix = () => new Date().getFullYear() + "-";
export const dueOrEmpty = (v: string) => (/^\d{4}-\d{2}-\d{2}$/.test(v) ? v : "");
export function DueInput({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { t } = useT();
  const ref = useRef<HTMLInputElement>(null);
  return (
    <span className="due">
      <input value={value} onChange={(e) => onChange(e.target.value)} inputMode="numeric" placeholder={yearPrefix() + "MM-DD"} pattern="\\d{4}-\\d{2}-\\d{2}|\\d{4}-?" title={t("YYYY-MM-DD, or leave the year alone for no date")} />
      <input ref={ref} type="date" tabIndex={-1} aria-hidden="true" onChange={(e) => e.target.value && onChange(e.target.value)} />
      <button type="button" className="ghost" aria-label={t("Pick a date")} onClick={() => { const el = ref.current; if (!el) return; (el as any).showPicker ? (el as any).showPicker() : el.click(); }}>📅</button>
    </span>
  );
}
