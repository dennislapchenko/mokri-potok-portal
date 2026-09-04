import { useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, canEdit } from "./shared";

// The message board half of the tavern. The calendar is the other half —
// Hall.tsx stacks them, because a village opens one room, not two.
export function Board({ me, items, reload }: { me: Me; items: any[]; reload: () => void }) {
  const { t } = useT();
  const [body, setBody] = useState("");
  const [author, setAuthor] = useState(() => { try { return localStorage.getItem("potok.author") || ""; } catch { return ""; } });
  const [replyTo, setReplyTo] = useState<number | null>(null);

  const post = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!body.trim()) return;
    try { localStorage.setItem("potok.author", author); } catch { /* ignore */ }
    await api("/posts", { method: "POST", body: { body, author, parent_id: replyTo ?? undefined } });
    setBody(""); setReplyTo(null); reload();
  };
  const roots = items.filter((p) => !p.parent_id);
  const replies = (id: number) => items.filter((p) => p.parent_id === id).sort((a, b) => a.created_at.localeCompare(b.created_at));

  return (
    <div className="parchment">
      <h2>📜 {t("The board")} <span className="sub">{t("what must stay findable")}</span></h2>
      <form className="inline" onSubmit={post}>
        <div className="row">
          <label>{t("Your name (optional)")}<input value={author} onChange={(e) => setAuthor(e.target.value)} maxLength={40} /></label>
        </div>
        <label>{replyTo ? t("Reply") : t("Write to the village")}<textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder={t("What is on your mind")} maxLength={4000} /></label>
        <div className="submit">
          {replyTo && <button type="button" className="ghost" onClick={() => setReplyTo(null)}>✕</button>}
          <button className="primary" type="submit">{replyTo ? t("Reply") : t("Post")}</button>
        </div>
      </form>
      {roots.length === 0 && <Empty text={t("No posts yet. Be the first.")} />}
      {roots.map((p) => (
        <div key={p.id} className={"card" + (p.pinned ? " pinned" : "")}>
          <div className="head"><Crest crest={p.house_crest} color={p.house_color} /><span className="who">{p.house_name}{p.author ? ` · ${p.author}` : ""}</span>{p.pinned ? <span className="tag alarm">📌 {t("Pinned")}</span> : null}<When iso={p.created_at} /></div>
          <div className="body">{p.body}</div>
          <div className="actions">
            <button className="ghost" onClick={() => setReplyTo(p.id)}>↩ {t("Reply")}</button>
            {me.is_steward === 1 && <button className="ghost" onClick={() => api(`/posts/${p.id}`, { method: "PUT", body: { pinned: !p.pinned } }).then(reload)}>📌 {p.pinned ? t("Unpin") : t("Pin")}</button>}
            {canEdit(me, p) && <button className="ghost" onClick={() => confirm("?") && api(`/posts/${p.id}`, { method: "DELETE" }).then(reload)}>🗑 {t("Delete")}</button>}
          </div>
          {replies(p.id).map((r) => (
            <div key={r.id} className="card" style={{ marginLeft: "1.5rem", borderLeftColor: "var(--parch3)" }}>
              <div className="head"><Crest crest={r.house_crest} color={r.house_color} /><span className="who">{r.house_name}{r.author ? ` · ${r.author}` : ""}</span><When iso={r.created_at} /></div>
              <div className="body">{r.body}</div>
              {canEdit(me, r) && <div className="actions"><button className="ghost" onClick={() => confirm("?") && api(`/posts/${r.id}`, { method: "DELETE" }).then(reload)}>🗑 {t("Delete")}</button></div>}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
