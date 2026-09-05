import { useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, useList } from "./shared";

// Campground: a camper's stay in three states. A house notices an arrival
// (optional note), a house says it has the money, then hands it over. Handed
// rows sink into the log, faded. No amounts, no plates — the box is the ledger.
export function Camp({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/camp");
  const [notes, setNotes] = useState("");
  const [have, setHave] = useState(false);
  const [claimFor, setClaimFor] = useState<number | null>(null);
  const [claimNote, setClaimNote] = useState("");

  const arrived = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/camp", { method: "POST", body: { notes, have_money: have } });
    setNotes(""); setHave(false); reload();
  };
  const claim = async (id: number) => {
    await api(`/camp/${id}`, { method: "PUT", body: { claim: true, notes: claimNote } });
    setClaimFor(null); setClaimNote(""); reload();
  };
  const put = (id: number, body: any) => api(`/camp/${id}`, { method: "PUT", body }).then(reload);

  const live = items.filter((x) => x.state !== "handed"), log = items.filter((x) => x.state === "handed");
  const Row = ({ x }: { x: any }) => {
    const holder = x.held_by === me.id;
    return (
      <div className={"card" + (x.state === "handed" ? " faded" : "")} style={{ borderLeftColor: x.state === "arrived" ? "var(--green)" : x.state === "held" ? "var(--brass)" : "var(--parch3)" }}>
        <div className="head">
          <Crest crest={x.house_crest} color={x.house_color} /><span className="who">{x.house_name}</span>
          <span className={"tag " + (x.state === "arrived" ? "open" : x.state === "held" ? "taken" : "done")}>
            {x.state === "arrived" ? "🏕️ " + t("arrived") : x.state === "held" ? `💰 ${x.held_by_crest} ${x.held_by_name}` : "✓ " + t("handed over")}
          </span>
          <When iso={x.taken_on} />
        </div>
        {x.notes && <div className="body small">{x.notes}</div>}
        {claimFor === x.id && (
          <div style={{ display: "flex", gap: ".4rem", margin: ".3rem 0" }}>
            <input value={claimNote} onChange={(e) => setClaimNote(e.target.value)} maxLength={300} placeholder={x.notes ? t("note already there") : t("a note, optional")} disabled={!!x.notes} autoFocus={!x.notes} />
            <button className="primary" onClick={() => claim(x.id)}>💰 {t("I have the money")}</button>
          </div>
        )}
        <div className="actions">
          {x.state === "arrived" && claimFor !== x.id && <button className="primary" onClick={() => (x.notes ? claim(x.id) : setClaimFor(x.id))}>💰 {t("I have the money")}</button>}
          {x.state === "held" && (holder || me.is_steward === 1) && <button onClick={() => put(x.id, { state: "handed" })}>✓ {t("Handed over")}</button>}
          {x.state === "handed" && (holder || me.is_steward === 1) && <button className="ghost" onClick={() => put(x.id, { state: "held" })}>{t("Still held")}</button>}
          {(x.house_id === me.id || holder || me.is_steward === 1) && <button className="ghost" onClick={() => confirm("?") && api(`/camp/${x.id}`, { method: "DELETE" }).then(reload)}>🗑</button>}
        </div>
      </div>
    );
  };

  return (
    <div className="parchment">
      <h2>🏕️ {t("Campground")} <span className="sub">{t("who noticed, who has the money")}</span></h2>
      <form className="inline" onSubmit={arrived}>
        <div className="row">
          <label style={{ gridColumn: "span 2" }}>{t("A camper arrived")}<input value={notes} onChange={(e) => setNotes(e.target.value)} maxLength={300} placeholder={t("a note, optional — grey camper, 2 nights, no plates")} /></label>
        </div>
        <label className="kind" style={{ marginTop: ".4rem" }}><input type="checkbox" checked={have} onChange={(e) => setHave(e.target.checked)} /> 💰 {t("I already have the money")}</label>
        <div className="submit"><button className="primary" type="submit">🏕️ {t("Camper arrived")}</button></div>
      </form>
      {live.length === 0 && log.length === 0 && <Empty text={t("Nothing yet. The first camper of the season will show up here.")} />}
      {live.map((x) => <Row key={x.id} x={x} />)}
      {log.length > 0 && <h3 style={{ marginTop: "1rem", color: "var(--ink2)" }}>📜 {t("Log of campers")}</h3>}
      {log.map((x) => <Row key={x.id} x={x} />)}
    </div>
  );
}
