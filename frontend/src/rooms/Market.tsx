import { useState } from "react";
import { api, type House, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, When, canEdit, useList } from "./shared";
import { DatePicker } from "../DatePicker";

export function Market({ me, houses }: { me: Me; houses: House[] }) {
  const { t } = useT();
  const [tab, setTab] = useState<"needs" | "offers" | "runs">("needs");
  const needs = useList("/needs"), offers = useList("/offers"), runs = useList("/runs");
  const [text, setText] = useState("");
  const [tag, setTag] = useState("giveaway");
  const [run, setRun] = useState({ destination: "", cutoff_at: "", notes: "" });
  const [runId, setRunId] = useState<string>("");
  const houseName = (id: number | null) => houses.find((h) => h.id === id)?.name;
  // One card edits at a time; the form takes the card's place, like an event's.
  const [editing, setEditing] = useState<string>("");
  const [ef, setEf] = useState<any>({});
  const startEdit = (key: string, fields: any) => { setEf(fields); setEditing(key); };
  const saveEdit = async (path: string, id: number, reload: () => void) => { await api(`${path}/${id}`, { method: "PUT", body: ef }); setEditing(""); reload(); };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (tab === "needs") { if (!text.trim()) return; await api("/needs", { method: "POST", body: { text, run_id: runId ? Number(runId) : undefined } }); setText(""); needs.reload(); }
    if (tab === "offers") { if (!text.trim()) return; await api("/offers", { method: "POST", body: { text, tag } }); setText(""); offers.reload(); }
    if (tab === "runs") { await api("/runs", { method: "POST", body: run }); setRun({ destination: "", cutoff_at: "", notes: "" }); runs.reload(); }
  };
  const setState = (path: string, id: number, state: string, reload: () => void) => api(`${path}/${id}`, { method: "PUT", body: { state } }).then(reload);

  // Render functions, not components: a component declared inside the room is
  // a new type on every render, so React would remount the card on each
  // keystroke and throw the caret to the end of the field being edited. The
  // lowercase name is the guard — <claimable /> does not typecheck.
  const claimable = (path: string, byCol: string, claimedState: "taken" | "claimed", reload: () => void) => (x: any) => {
    const mine = x.house_id === me.id;
    const claimer = (x.taken_by ?? x.claimed_by) === me.id;
    const byName: string | undefined = x[byCol];
    return (
      <div key={x.id} className="card" style={{ opacity: x.state === "done" ? 0.55 : 1 }}>
        {editing === path + x.id ? (
          <form className="inline" onSubmit={(e) => { e.preventDefault(); saveEdit(path, x.id, reload); }}>
            <div className="row">
              <label style={x.tag ? { gridColumn: "span 2" } : undefined}>{t(path === "/needs" ? "I need" : "I give away")}<input value={ef.text} onChange={(e) => setEf({ ...ef, text: e.target.value })} required maxLength={300} autoFocus /></label>
              {x.tag && <label>{t("Kind")}<select value={ef.tag} onChange={(e) => setEf({ ...ef, tag: e.target.value })}>{["giveaway", "seeds", "surplus", "joint"].map((k) => <option key={k} value={k}>{t(k)}</option>)}</select></label>}
            </div>
            <div className="submit"><button type="button" className="ghost" onClick={() => setEditing("")}>✕</button><button className="primary" type="submit">{t("Save")}</button></div>
          </form>
        ) : (<>
        <div className="head"><Crest crest={x.house_crest} color={x.house_color} /><span className="who">{x.house_name}</span><span className={"tag " + x.state}>{t(x.state)}</span>{x.tag && <span className="tag">{t(x.tag)}</span>}<When iso={x.created_at} /></div>
        <div className="body">{x.text}{x.run_id && <span className="small"> · 🚗 {houseName(runs.items.find((r) => r.id === x.run_id)?.house_id) || ""}</span>}</div>
        {byName && x.state !== "done" && <div className="small">{t(claimedState === "taken" ? "Taken by" : "Claimed by")}: <strong>{byName}</strong></div>}
        </>)}
        <div className="actions">
          {x.state === "open" && !mine && <button className="primary" onClick={() => setState(path, x.id, claimedState, reload)}>{claimedState === "taken" ? "🛒 " + t("I take it") : "🙋 " + t("I want it")}</button>}
          {x.state === claimedState && (claimer || canEdit(me, x)) && <button onClick={() => setState(path, x.id, "open", reload)}>{t("Release")}</button>}
          {x.state !== "done" && (claimer || canEdit(me, x)) && <button onClick={() => setState(path, x.id, "done", reload)}>✓ {t("Done")}</button>}
          {canEdit(me, x) && editing !== path + x.id && <button className="ghost" onClick={() => startEdit(path + x.id, x.tag ? { text: x.text, tag: x.tag } : { text: x.text })}>✎ {t("Edit")}</button>}
          {canEdit(me, x) && <button className="ghost" onClick={() => confirm("?") && api(`${path}/${x.id}`, { method: "DELETE" }).then(reload)}>🗑</button>}
        </div>
      </div>
    );
  };

  return (
    <div className="parchment">
      <h2>🧺 {t("Market")} <span className="sub">{t("needs, give-aways, runs")}</span></h2>
      <div className="tabs">
        <button className={tab === "needs" ? "active" : ""} onClick={() => setTab("needs")}>🛒 {t("Needs")} ({needs.items.filter((x) => x.state === "open").length})</button>
        <button className={tab === "offers" ? "active" : ""} onClick={() => setTab("offers")}>🎁 {t("Give-aways")} ({offers.items.filter((x) => x.state === "open").length})</button>
        <button className={tab === "runs" ? "active" : ""} onClick={() => setTab("runs")}>🚗 {t("Runs")} ({runs.items.length})</button>
      </div>
      <form className="inline" onSubmit={submit}>
        {tab === "needs" && (<>
          <label>{t("I need")}<input value={text} onChange={(e) => setText(e.target.value)} placeholder={t("What do you need from the shop")} maxLength={300} /></label>
          {runs.items.length > 0 && <label>🚗<select value={runId} onChange={(e) => setRunId(e.target.value)}><option value="">—</option>{runs.items.map((r) => <option key={r.id} value={r.id}>{r.house_name} → {r.destination}</option>)}</select></label>}
        </>)}
        {tab === "offers" && (
          <div className="row">
            <label style={{ gridColumn: "span 2" }}>{t("I give away")}<input value={text} onChange={(e) => setText(e.target.value)} placeholder={t("What you have and do not need")} maxLength={300} /></label>
            <label>{t("Kind")}<select value={tag} onChange={(e) => setTag(e.target.value)}>{["giveaway", "seeds", "surplus", "joint"].map((k) => <option key={k} value={k}>{t(k)}</option>)}</select></label>
          </div>
        )}
        {tab === "runs" && (
          <div className="row">
            <label>{t("Destination")}<input value={run.destination} onChange={(e) => setRun({ ...run, destination: e.target.value })} required maxLength={120} /></label>
            <label>{t("Leaving at")}<DatePicker time required value={run.cutoff_at} onChange={(v) => setRun({ ...run, cutoff_at: v })} /></label>
            <label>{t("Notes")}<input value={run.notes} onChange={(e) => setRun({ ...run, notes: e.target.value })} maxLength={300} /></label>
          </div>
        )}
        <div className="submit"><button className="primary" type="submit">{tab === "runs" ? t("Post the run") : t("Post")}</button></div>
      </form>
      {tab === "needs" && (needs.items.length ? needs.items.map(claimable("/needs", "taken_by_name", "taken", needs.reload)) : <Empty text={t("Nothing here yet.")} />)}
      {tab === "offers" && (offers.items.length ? offers.items.map(claimable("/offers", "claimed_by_name", "claimed", offers.reload)) : <Empty text={t("Nothing here yet.")} />)}
      {tab === "runs" && (runs.items.length ? runs.items.map((r) => (
        <div key={r.id} className="card">
          {editing === "/runs" + r.id ? (
            <form className="inline" onSubmit={(e) => { e.preventDefault(); saveEdit("/runs", r.id, runs.reload); }}>
              <div className="row">
                <label>{t("Destination")}<input value={ef.destination} onChange={(e) => setEf({ ...ef, destination: e.target.value })} required maxLength={120} autoFocus /></label>
                <label>{t("Leaving at")}<DatePicker time required value={ef.cutoff_at} onChange={(v) => setEf({ ...ef, cutoff_at: v })} /></label>
                <label>{t("Notes")}<input value={ef.notes} onChange={(e) => setEf({ ...ef, notes: e.target.value })} maxLength={300} /></label>
              </div>
              <div className="submit"><button type="button" className="ghost" onClick={() => setEditing("")}>✕</button><button className="primary" type="submit">{t("Save")}</button></div>
            </form>
          ) : (<>
          <div className="head"><Crest crest={r.house_crest} color={r.house_color} /><span className="who">{r.house_name}</span> {t("drives to")} <strong>{r.destination}</strong><span className="when">{t("leaving")} <When iso={r.cutoff_at} /></span></div>
          {r.notes && <div className="body small">{r.notes}</div>}
          </>)}
          <div className="small">{needs.items.filter((n) => n.run_id === r.id && n.state !== "done").map((n) => <div key={n.id}>• {n.house_name}: {n.text}</div>)}</div>
          {canEdit(me, r) && <div className="actions">
            {editing !== "/runs" + r.id && <button className="ghost" onClick={() => startEdit("/runs" + r.id, { destination: r.destination, cutoff_at: r.cutoff_at, notes: r.notes || "" })}>✎ {t("Edit")}</button>}
            <button className="ghost" onClick={() => confirm("?") && api(`/runs/${r.id}`, { method: "DELETE" }).then(runs.reload)}>🗑</button>
          </div>}
        </div>
      )) : <Empty text={t("Nothing here yet.")} />)}
    </div>
  );
}
