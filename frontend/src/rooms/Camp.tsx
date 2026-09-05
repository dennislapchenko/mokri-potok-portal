import { useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, canEdit, useList } from "./shared";

// Campground: one row = one house collected from one camper, and a note. No
// amounts — the cash box holds the money, this holds who has it until it is
// handed over. From-who is a label a villager would say aloud, never a plate.
export function Camp({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/camp");
  const today = new Date().toISOString().slice(0, 10);
  const [f, setF] = useState({ taken_on: today, from_who: "", collected_by: "", notes: "" });
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/camp", { method: "POST", body: f });
    setF({ taken_on: today, from_who: "", collected_by: "", notes: "" }); reload();
  };
  const held = items.filter((x) => x.state === "held");
  return (
    <div className="parchment">
      <h2>🏕️ {t("Campground")} <span className="sub">{t("who collected from whom")}</span></h2>
      <p className="small">{t("No amounts here — the cash box is the ledger. \"Handed over\" means this house no longer holds it; to whom is still to be agreed by the collective.")}</p>
      <form className="inline" onSubmit={save}>
        <div className="row">
          <label>{t("Date")}<input type="date" value={f.taken_on} onChange={(e) => setF({ ...f, taken_on: e.target.value })} required /></label>
          <label>{t("From whom")}<input value={f.from_who} onChange={(e) => setF({ ...f, from_who: e.target.value })} required maxLength={80} placeholder={t("grey camper, family from NL — no plates")} /></label>
          <label>{t("Collected by (name, optional)")}<input value={f.collected_by} onChange={(e) => setF({ ...f, collected_by: e.target.value })} maxLength={40} /></label>
          <label>{t("Notes")}<input value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} maxLength={300} placeholder={t("2 nights, paid cash")} /></label>
        </div>
        <div className="submit"><button className="primary" type="submit">💰 {t("I collected")}</button></div>
      </form>
      {held.length > 0 && <p className="small">{t("Held, not yet handed over:")} {held.map((x) => `${x.house_crest} ${x.house_name}`).filter((v, i, a) => a.indexOf(v) === i).join(", ")}</p>}
      {items.length === 0 && <Empty text={t("Nothing collected yet.")} />}
      {items.map((x) => (
        <div key={x.id} className="card" style={{ opacity: x.state === "handed" ? 0.7 : 1, borderLeftColor: x.state === "handed" ? "var(--parch3)" : "var(--brass)" }}>
          <div className="head">
            <Crest crest={x.house_crest} color={x.house_color} /><span className="who">{x.collected_by ? `${x.collected_by} · ` : ""}{x.house_name}</span>
            <strong>{x.from_who}</strong>
            <span className={"tag " + (x.state === "handed" ? "done" : "taken")}>{x.state === "handed" ? "✓ " + t("handed over") : t("held")}</span>
            <When iso={x.taken_on} />
          </div>
          {x.notes && <div className="body small">{x.notes}</div>}
          {canEdit(me, x) && (
            <div className="actions">
              {x.state === "held" ? <button onClick={() => api(`/camp/${x.id}`, { method: "PUT", body: { state: "handed" } }).then(reload)}>✓ {t("Handed over")}</button>
                : <button className="ghost" onClick={() => api(`/camp/${x.id}`, { method: "PUT", body: { state: "held" } }).then(reload)}>{t("Still held")}</button>}
              <button className="ghost" onClick={() => confirm("?") && api(`/camp/${x.id}`, { method: "DELETE" }).then(reload)}>🗑</button>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
