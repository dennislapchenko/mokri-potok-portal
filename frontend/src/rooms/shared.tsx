import { useCallback, useEffect, useState } from "react";
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

// stamp: local "YYYY-MM-DDTHH:MM", the same shape every date in the app has,
// so events can be compared as plain strings without a timezone conversion.
export const stamp = (d = new Date()) => {
  const n2 = (v: number) => String(v).padStart(2, "0");
  return `${d.getFullYear()}-${n2(d.getMonth() + 1)}-${n2(d.getDate())}T${n2(d.getHours())}:${n2(d.getMinutes())}`;
};

// isOver: an event is over when its END has passed, not its start. An event
// with no end time has no known length, so it lasts until the end of its day —
// dropping a work party from the list one minute after it begins would hide it
// from exactly the person who is running late. A past event never disappears:
// it stays in its day cell in the calendar, one click away.
export function isOver(ev: { starts_at: string; ends_at?: string | null }, now = stamp()): boolean {
  const end = ev.ends_at || ev.starts_at;
  return (end.length <= 10 ? end + "T23:59" : end) < now;
}

export const canEdit = (me: { id: number; is_steward: number }, row: { house_id: number }) => row.house_id === me.id || me.is_steward === 1;
