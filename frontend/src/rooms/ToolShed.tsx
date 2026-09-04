import { useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, canEdit, useList } from "./shared";

// Tool shed: what the village lends. A tool has an owner house and, while it is
// out, a holder. No counting of who borrows most — the reciprocity stays social.
export function ToolShed({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/tools");
  const [f, setF] = useState({ name: "", notes: "" });

  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!f.name.trim()) return;
    await api("/tools", { method: "POST", body: f });
    setF({ name: "", notes: "" });
    reload();
  };
  const take = (id: number, v: boolean) => api(`/tools/${id}`, { method: "PUT", body: { take: v } }).then(reload);

  const free = items.filter((x) => !x.held_by);
  const out = items.filter((x) => x.held_by);

  const Tool = ({ x }: { x: any }) => (
    <div className="card" style={{ borderLeftColor: x.held_by ? "var(--parch3)" : "var(--green)" }}>
      <div className="head">
        <Crest crest={x.house_crest} color={x.house_color} />
        <strong>{x.name}</strong>
        <span className="small">{x.house_name}</span>
        {x.held_by ? <span className="tag taken">{x.held_by_crest} {x.held_by_name}</span> : <span className="tag open">{t("in the shed")}</span>}
        {x.held_since && <span className="when"><When iso={x.held_since} /></span>}
      </div>
      {x.notes && <div className="body small">{x.notes}</div>}
      <div className="actions">
        {!x.held_by && <button className="primary" onClick={() => take(x.id, true)}>🤲 {t("I take it")}</button>}
        {x.held_by === me.id && <button onClick={() => take(x.id, false)}>↩ {t("I brought it back")}</button>}
        {x.held_by && x.held_by !== me.id && canEdit(me, x) && <button onClick={() => take(x.id, false)}>↩ {t("Mark returned")}</button>}
        {canEdit(me, x) && <button className="ghost" onClick={() => confirm(x.name + "?") && api(`/tools/${x.id}`, { method: "DELETE" }).then(reload)}>🗑</button>}
      </div>
    </div>
  );

  return (
    <div className="parchment">
      <h2>🛠 {t("Tool shed")} <span className="sub">{t("what the village lends")}</span></h2>
      <form className="inline" onSubmit={add}>
        <div className="row">
          <label>{t("I share a tool")}<input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} placeholder={t("chainsaw, ladder, trailer…")} maxLength={80} /></label>
          <label>{t("Notes")}<input value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} placeholder={t("bring your own fuel; ask first")} maxLength={200} /></label>
        </div>
        <div className="submit"><button className="primary" type="submit">{t("Put in the shed")}</button></div>
      </form>
      {items.length === 0 && <Empty text={t("The shed is empty. Put something in it.")} />}
      {free.map((x) => <Tool key={x.id} x={x} />)}
      {out.length > 0 && <h3 style={{ marginTop: "1rem", color: "var(--ink2)" }}>{t("Out on loan")}</h3>}
      {out.map((x) => <Tool key={x.id} x={x} />)}
    </div>
  );
}
