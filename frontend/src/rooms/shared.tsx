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

export const canEdit = (me: { id: number; is_steward: number }, row: { house_id: number }) => row.house_id === me.id || me.is_steward === 1;
