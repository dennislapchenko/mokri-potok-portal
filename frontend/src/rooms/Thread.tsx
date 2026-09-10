import { useEffect, useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, When } from "./shared";

// One comment thread, wherever a room wants one: an event, a wish, an away
// notice, a number in the phone book. The board's own shape, one reply level. The name field starts filled
// with the label this phone joined under, so most people never type it.
export function Thread({ subject, id, me, onChanged }: { subject: "event" | "wish" | "away" | "contact"; id: number; me: Me; onChanged?: () => void }) {
  const { t } = useT();
  const [items, setItems] = useState<any[]>([]);
  const [body, setBody] = useState("");
  const [replyTo, setReplyTo] = useState<number | null>(null);
  const [author, setAuthor] = useState(() => defaultAuthor(me));
  const load = () => api<any[]>(`/threads/${subject}/${id}`).then(setItems).catch(() => setItems([]));
  useEffect(() => { load(); }, [subject, id]); // eslint-disable-line react-hooks/exhaustive-deps

  const post = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!body.trim()) return;
    try { localStorage.setItem("potok.author", author); } catch { /* ignore */ }
    await api(`/threads/${subject}/${id}`, { method: "POST", body: { body, author, parent_id: replyTo ?? undefined } });
    setBody(""); setReplyTo(null); load(); onChanged?.();
  };
  const del = (cid: number) => confirm("?") && api(`/comments/${cid}`, { method: "DELETE" }).then(() => { load(); onChanged?.(); });
  const roots = items.filter((c) => !c.parent_id);
  const replies = (cid: number) => items.filter((c) => c.parent_id === cid);

  // A render function, not a component — see Market.tsx: as a component it
  // remounted every comment card on each keystroke in the reply box.
  const one = (c: any, reply?: boolean) => (
    <div key={c.id} className="card comment" style={reply ? { marginLeft: "1.4rem" } : undefined}>
      <div className="head"><Crest crest={c.house_crest} color={c.house_color} /><span className="who">{c.author || c.house_name}</span><When iso={c.created_at} /></div>
      <div className="body">{c.body}</div>
      <div className="actions">
        {!reply && <button className="ghost" onClick={() => setReplyTo(c.id)}>↩ {t("Reply")}</button>}
        {(c.house_id === me.id || me.is_steward === 1) && <button className="ghost" onClick={() => del(c.id)}>🗑</button>}
      </div>
    </div>
  );

  return (
    <div className="comments">
      <form className="inline" onSubmit={post}>
        <div className="row"><label>{t("Your name (optional)")}<input value={author} onChange={(e) => setAuthor(e.target.value)} maxLength={40} /></label></div>
        <label>{replyTo ? t("Reply") : t("Comment")}<textarea value={body} onChange={(e) => setBody(e.target.value)} maxLength={2000} /></label>
        <div className="submit">{replyTo && <button type="button" className="ghost" onClick={() => setReplyTo(null)}>✕</button>}<button className="lesser" type="submit">{replyTo ? t("Reply") : t("Post")}</button></div>
      </form>
      {roots.length === 0 && <p className="small muted" style={{ fontStyle: "italic" }}>{t("No comments yet.")}</p>}
      {roots.map((c) => (<div key={c.id}>{one(c)}{replies(c.id).map((r) => one(r, true))}</div>))}
    </div>
  );
}

// The name to start with: what this person typed last, else the label this
// phone joined under ("Ana's phone" → "Ana").
export function defaultAuthor(me: Me): string {
  try {
    const saved = localStorage.getItem("potok.author");
    if (saved) return saved;
  } catch { /* ignore */ }
  const label = (me as any).device_label as string | undefined;
  if (!label) return "";
  return label.replace(/[’']s\s+(phone|telefon).*$/i, "").replace(/\s*(phone|telefon|naprava|laptop|prenosnik|iphone|android)\s*$/i, "").trim();
}
