import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, type House, type Me } from "../api";
import { useT } from "../i18n";
import { photoURL, uploadPhoto } from "../photo";
import { Crest, Empty, When, canEdit, useList } from "./shared";
import { Thread } from "./Thread";

// Tool shed: what the village lends. A tool has an owner house and, while it is
// out, a holder. No counting of who borrows most — the reciprocity stays social.
// Filters by owner house and by category; a wishlist folded at the bottom.

export const CATEGORIES = [
  { id: "power", icon: "⚡", name: "Power tools" },
  { id: "garden", icon: "🌱", name: "Gardening" },
  { id: "other", icon: "🧰", name: "Other" },
];

function Photo({ id, version, onOpen }: { id: number; version: number; onOpen: (url: string) => void }) {
  const [url, setUrl] = useState<string>("");
  useEffect(() => { photoURL(id, version).then(setUrl).catch(() => setUrl("")); }, [id, version]);
  if (!url) return <div className="tool-photo empty" />;
  return <img className="tool-photo" src={url} alt="" onClick={() => onOpen(url)} />;
}

export function ToolShed({ me, houses }: { me: Me; houses: House[] }) {
  const { t } = useT();
  const { items, reload } = useList("/tools");
  const wishes = useList("/wishes");
  const [f, setF] = useState({ name: "", notes: "", category: "other" });
  const [file, setFile] = useState<File | null>(null);
  const [owner, setOwner] = useState<number | null>(null);
  const [cat, setCat] = useState<string | null>(null);
  const [big, setBig] = useState<string>("");
  const [ver, setVer] = useState<Record<number, number>>({});
  const [wish, setWish] = useState("");
  const [optFor, setOptFor] = useState<number | null>(null);
  const [opt, setOpt] = useState({ text: "", url: "" });
  const [threadFor, setThreadFor] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const [adding, setAdding] = useState(false);

  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!f.name.trim()) return;
    setBusy(true);
    try {
      const { id } = await api<{ id: number }>("/tools", { method: "POST", body: f });
      if (file) await uploadPhoto(id, file);
      setF({ name: "", notes: "", category: "other" }); setFile(null); if (fileRef.current) fileRef.current.value = "";
      setAdding(false); reload();
    } finally { setBusy(false); }
  };
  const take = (id: number, v: boolean) => api(`/tools/${id}`, { method: "PUT", body: { take: v } }).then(reload);
  const replacePhoto = async (id: number, fl: File | null) => {
    if (!fl) return;
    await uploadPhoto(id, fl);
    setVer((v) => ({ ...v, [id]: (v[id] || 0) + 1 }));
    reload();
  };

  const owners = useMemo(() => houses.filter((h) => items.some((x) => x.house_id === h.id)), [houses, items]);
  const shown = items.filter((x) => (owner === null || x.house_id === owner) && (cat === null || x.category === cat));
  const free = shown.filter((x) => !x.held_by), out = shown.filter((x) => x.held_by);
  const catOf = (id: string) => CATEGORIES.find((c) => c.id === id) || CATEGORIES[2];

  const Tool = ({ x }: { x: any }) => (
    <div className={"card tool" + (x.held_by ? " out" : "")} style={{ borderLeftColor: x.held_by ? "var(--parch3)" : "var(--green)" }}>
      <div className="tool-main">
        <div className="head">
          <Link to="/houses" title={x.house_name} className="crest-link"><Crest crest={x.house_crest} color={x.house_color} /></Link>
          <strong>{x.name}</strong>
          <span className="tag">{catOf(x.category).icon} {t(catOf(x.category).name)}</span>
          {x.held_by ? <span className="tag taken">{x.held_by_crest} {x.held_by_name}</span> : <span className="tag open">{t("in the shed")}</span>}
        </div>
        {(x.held_since || x.notes) && <div className="small">{x.held_since ? <When iso={x.held_since} /> : null}{x.held_since && x.notes ? " · " : ""}{x.notes}</div>}
        <div className="actions">
          {!x.held_by && <button className="primary" onClick={() => take(x.id, true)}>🤲 {t("I take it")}</button>}
          {x.held_by === me.id && <button onClick={() => take(x.id, false)}>↩ {t("I brought it back")}</button>}
          {x.held_by && x.held_by !== me.id && canEdit(me, x) && <button onClick={() => take(x.id, false)}>↩ {t("Mark returned")}</button>}
          {canEdit(me, x) && (
            <label className="btn-file">📷<input type="file" accept="image/*" capture="environment" onChange={(e) => replacePhoto(x.id, e.target.files?.[0] || null)} /></label>
          )}
          {canEdit(me, x) && <button className="ghost" onClick={() => confirm(x.name + "?") && api(`/tools/${x.id}`, { method: "DELETE" }).then(reload)}>🗑</button>}
        </div>
      </div>
      {x.has_photo ? <Photo id={x.id} version={ver[x.id] || 0} onOpen={setBig} /> : null}
    </div>
  );

  return (
    <>
      <div className="parchment">
        <h2>🛠 {t("Tool shed")} <span className="sub">{t("what the village has")}</span><button style={{ marginLeft: "auto" }} onClick={() => setAdding(!adding)}>+ {t("I share a tool")}</button></h2>
        <p className="small muted" style={{ fontStyle: "italic", marginTop: "-.3rem" }}>{t("Take and return are for when it helps. A tool may simply be listed, so the village knows it exists.")}</p>
        {adding && <form className="inline" onSubmit={add}>
          <div className="row">
            <label>{t("I share a tool")}<input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} placeholder={t("chainsaw, ladder, trailer…")} maxLength={80} /></label>
            <label>{t("Category")}<select value={f.category} onChange={(e) => setF({ ...f, category: e.target.value })}>{CATEGORIES.map((c) => <option key={c.id} value={c.id}>{c.icon} {t(c.name)}</option>)}</select></label>
            <label>{t("Notes")}<input value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} placeholder={t("bring your own fuel; ask first")} maxLength={200} /></label>
            <label>{t("Photo")}<input ref={fileRef} type="file" accept="image/*" capture="environment" onChange={(e) => setFile(e.target.files?.[0] || null)} /></label>
          </div>
          <div className="submit"><button className="primary" type="submit" disabled={busy}>{t("Put in the shed")}</button></div>
        </form>}

        {items.length > 0 && (
          <div className="chip-rows">
            <div className="chip-row">
              <span className="chip-label">{t("who")}</span>
              <div className="chips">
                <button className={"chip" + (owner === null ? " on" : "")} onClick={() => setOwner(null)}>{t("Everyone")}</button>
                {owners.map((h) => <button key={h.id} className={"chip" + (owner === h.id ? " on" : "")} onClick={() => setOwner(owner === h.id ? null : h.id)}>{h.crest} {h.name}</button>)}
              </div>
            </div>
            <div className="chip-row">
              <span className="chip-label">{t("what")}</span>
              <div className="chips">
                {CATEGORIES.map((c) => <button key={c.id} className={"chip" + (cat === c.id ? " on" : "")} onClick={() => setCat(cat === c.id ? null : c.id)}>{c.icon} {t(c.name)}</button>)}
              </div>
            </div>
          </div>
        )}
        {items.length === 0 && <Empty text={t("The shed is empty. Put something in it.")} />}
        {items.length > 0 && shown.length === 0 && <Empty text={t("Nothing here yet.")} />}
        {free.map((x) => <Tool key={x.id} x={x} />)}
        {out.length > 0 && <h3 style={{ marginTop: "1rem", color: "var(--ink2)" }}>{t("Out on loan")}</h3>}
        {out.map((x) => <Tool key={x.id} x={x} />)}
      </div>

      <details className="parchment wish">
        <summary>✨ {t("Wishlist")} <span className="sub">{t("tools the village lacks")}</span>{wishes.items.length > 0 && <span className="small"> · {wishes.items.length}</span>}</summary>
        <form className="inline" onSubmit={async (e) => { e.preventDefault(); if (!wish.trim()) return; await api("/wishes", { method: "POST", body: { text: wish } }); setWish(""); wishes.reload(); }}>
          <div className="row"><label>{t("I wish the village had")}<input value={wish} onChange={(e) => setWish(e.target.value)} maxLength={120} placeholder={t("log splitter, cider press…")} /></label></div>
          <div className="submit"><button type="submit">{t("Add a wish")}</button></div>
        </form>
        {wishes.items.map((w: any) => {
          const wants: any[] = JSON.parse(w.wants || "[]");
          return (
            <div key={w.id} className="card">
              <div className="head"><Crest crest={w.house_crest} color={w.house_color} /><strong>{w.text}</strong><When iso={w.created_at} /></div>
              <div className="small">🙋 {wants.map((h) => `${h.crest} ${h.name}`).join(", ")}</div>
              {(JSON.parse(w.options || "[]") as any[]).map((o) => (
                <div key={o.id} className="wish-option">
                  <Crest crest={o.crest} color={o.color} />
                  {o.url ? <a href={o.url} target="_blank" rel="noreferrer noopener">{o.text}</a> : <span>{o.text}</span>}
                  {(o.house_id === me.id || me.is_steward === 1) && <button className="ghost" onClick={() => confirm("?") && api(`/options/${o.id}`, { method: "DELETE" }).then(wishes.reload)}>🗑</button>}
                </div>
              ))}
              {optFor === w.id && (
                <form className="inline" onSubmit={async (e) => { e.preventDefault(); if (!opt.text.trim()) return; await api(`/wishes/${w.id}/options`, { method: "POST", body: opt }); setOpt({ text: "", url: "" }); setOptFor(null); wishes.reload(); }}>
                  <div className="row">
                    <label>{t("What you found")}<input value={opt.text} onChange={(e) => setOpt({ ...opt, text: e.target.value })} maxLength={200} placeholder={t("model, price, where")} autoFocus /></label>
                    <label>{t("Link (optional)")}<input value={opt.url} onChange={(e) => setOpt({ ...opt, url: e.target.value })} maxLength={500} /></label>
                  </div>
                  <div className="submit"><button type="button" className="ghost" onClick={() => setOptFor(null)}>✕</button><button className="lesser" type="submit">{t("Add")}</button></div>
                </form>
              )}
              <div className="actions">
                <button className={w.mine ? "" : "primary"} onClick={() => api(`/wishes/${w.id}`, { method: "PUT", body: { want: !w.mine } }).then(wishes.reload)}>{w.mine ? t("Not any more") : "🙋 " + t("I would love that too")}</button>
                {optFor !== w.id && <button className="lesser" onClick={() => setOptFor(w.id)}>🔎 {t("I found one")}</button>}
                <button className="ghost" onClick={() => setThreadFor(threadFor === w.id ? null : w.id)}>💬 {t("Comments")} ({w.comments || 0})</button>
                {canEdit(me, w) && <button className="ghost" onClick={() => confirm(w.text + "?") && api(`/wishes/${w.id}`, { method: "DELETE" }).then(wishes.reload)}>✓ {t("It arrived")}</button>}
              </div>
              {threadFor === w.id && <Thread subject="wish" id={w.id} me={me} onChanged={wishes.reload} />}
            </div>
          );
        })}
      </details>

      {big && <div className="lightbox" onClick={() => setBig("")}><img src={big} alt="" /></div>}
    </>
  );
}
