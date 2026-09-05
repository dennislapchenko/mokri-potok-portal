import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { EventCard } from "./Calendar";
import { Crest, Empty, When, canEdit, useList } from "./shared";

// Projects: a long job split into tasks. A task is taken, never handed to a
// house. Done is a state that stays visible. "3 of 5" is a project's progress,
// never a house's score.

export function Projects({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/projects");
  const [open, setOpen] = useState(false);
  const today = new Date().toISOString().slice(0, 10);
  const [f, setF] = useState({ title: "", due_at: today, notes: "" });
  const nav = useNavigate();

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    const { id } = await api<{ id: number }>("/projects", { method: "POST", body: f });
    setF({ title: "", due_at: today, notes: "" }); setOpen(false); reload();
    nav(`/projects/${id}`);
  };
  const live = items.filter((p) => p.state === "open"), done = items.filter((p) => p.state === "done");
  const Card = ({ p }: { p: any }) => (
    <Link to={`/projects/${p.id}`} className="card project" style={{ opacity: p.state === "done" ? 0.7 : 1 }}>
      <div className="head"><Crest crest={p.house_crest} color={p.house_color} /><strong>📋 {p.title}</strong>{p.due_at && <span className="when">{t("by")} <When iso={p.due_at} /></span>}</div>
      <div className="small">
        {p.tasks > 0 ? `${p.tasks_done} / ${p.tasks} ${t("tasks done")}` : t("no tasks yet")}
        {p.tasks_free > 0 && <> · <span className="tag open">{p.tasks_free} {t("free to take")}</span></>}
        {p.next_event && <> · 🔔 <When iso={p.next_event} /></>}
      </div>
    </Link>
  );
  return (
    <div className="parchment">
      <h2>📋 {t("Projects")} <span className="sub">{t("long jobs, split into tasks")}</span><button style={{ marginLeft: "auto" }} onClick={() => setOpen(!open)}>+ {t("New project")}</button></h2>
      {open && (
        <form className="inline" onSubmit={create}>
          <div className="row">
            <label>{t("Title")}<input value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} required maxLength={120} /></label>
            <label>{t("Due")}<input type="date" value={f.due_at} onChange={(e) => setF({ ...f, due_at: e.target.value })} /></label>
          </div>
          <label>{t("Notes")}<textarea value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} maxLength={2000} /></label>
          <div className="submit"><button className="primary" type="submit">{t("Create")}</button></div>
        </form>
      )}
      {items.length === 0 && <Empty text={t("No projects yet. Start one — a fence, a roof, a road.")} />}
      {live.map((p) => <Card key={p.id} p={p} />)}
      {done.length > 0 && <details style={{ marginTop: "1rem" }}><summary className="small">✓ {t("Finished")} ({done.length})</summary>{done.map((p) => <Card key={p.id} p={p} />)}</details>}
    </div>
  );
}

export function Project({ me, houses: allHouses }: { me: Me; houses: { id: number; name: string; crest: string; kind?: string }[] }) {
  const houses = allHouses.filter((h) => h.kind !== "common"); // land does not take tasks
  const { t } = useT();
  const { id } = useParams();
  const nav = useNavigate();
  const [p, setP] = useState<any>(null);
  const todayIso = new Date().toISOString().slice(0, 10);
  const [tf, setTf] = useState({ title: "", due_at: todayIso, notes: "" });
  const [addTask, setAddTask] = useState(false);
  const [closing, setClosing] = useState<number | null>(null);
  const [note, setNote] = useState("");
  const load = () => api<any>(`/projects/${id}`).then(setP).catch(() => setP(null));
  useEffect(() => { load(); }, [id]); // eslint-disable-line react-hooks/exhaustive-deps
  if (!p) return <div className="parchment">{t("Loading…")}</div>;

  const editable = canEdit(me, p);
  const tasks: any[] = p.tasks || [], events: any[] = p.events || [];
  const openTasks = tasks.filter((x) => x.state === "open"), doneTasks = tasks.filter((x) => x.state === "done");
  const today = new Date().toISOString().slice(0, 10);
  const put = (path: string, body: any) => api(path, { method: "PUT", body }).then(load);

  const Task = ({ x }: { x: any }) => {
    const mine = x.assigned_to === me.id;
    const creator = x.house_id === me.id || editable;
    return (
      <div className="card" style={{ opacity: x.state === "done" ? 0.7 : 1, borderLeftColor: x.state === "done" ? "var(--parch3)" : x.assigned_to ? "var(--brass)" : "var(--green)" }}>
        <div className="head">
          <strong>{x.state === "done" ? "✓ " : ""}{x.title}</strong>
          {x.assigned_to ? <span className="tag taken">{x.assigned_crest} {x.assigned_name}</span> : x.state === "open" ? <span className="tag open">{t("free to take")}</span> : null}
          {x.due_at && <span className="when">{t("by")} <When iso={x.due_at} /></span>}
        </div>
        {(x.notes || x.closing_note) && <div className="body small">{x.notes}{x.closing_note && <div>📝 {x.closing_note}</div>}</div>}
        {closing === x.id && (
          <div style={{ display: "flex", gap: ".4rem", margin: ".3rem 0" }}>
            <input value={note} onChange={(e) => setNote(e.target.value)} maxLength={300} placeholder={t("one line on how it went (optional)")} autoFocus />
            <button className="primary" onClick={() => put(`/tasks/${x.id}`, { state: "done", closing_note: note }).then(() => { setClosing(null); setNote(""); })}>✓ {t("Done")}</button>
          </div>
        )}
        <div className="actions">
          {x.state === "open" && !x.assigned_to && <button className="primary" onClick={() => put(`/tasks/${x.id}`, { take: true })}>🙋 {t("I take it")}</button>}
          {x.state === "open" && mine && <button onClick={() => put(`/tasks/${x.id}`, { take: false })}>{t("Let it go")}</button>}
          {x.state === "open" && creator && !mine && x.assigned_to && <button className="ghost" onClick={() => put(`/tasks/${x.id}`, { take: false })}>{t("Clear")}</button>}
          {x.state === "open" && creator && (
            <select className="assign" value="" onChange={(e) => e.target.value && put(`/tasks/${x.id}`, { assigned_to: Number(e.target.value) })}>
              <option value="">🤝 {t("Hand to")}…</option>
              {houses.filter((h) => h.id !== x.assigned_to).map((h) => <option key={h.id} value={h.id}>{h.crest} {h.name}</option>)}
            </select>
          )}
          {x.state === "open" && (mine || creator) && closing !== x.id && <button onClick={() => setClosing(x.id)}>✓ {t("Done")}</button>}
          {x.state === "done" && (mine || creator) && <button className="ghost" onClick={() => put(`/tasks/${x.id}`, { state: "open" })}>{t("Reopen")}</button>}
          {creator && <button className="ghost" onClick={() => confirm(x.title + "?") && api(`/tasks/${x.id}`, { method: "DELETE" }).then(load)}>🗑</button>}
        </div>
      </div>
    );
  };

  return (
    <>
      <div className="parchment">
        <p className="small"><Link to="/projects">← {t("Projects")}</Link></p>
        <h2>📋 {p.title} {p.state === "done" && <span className="tag done">✓ {t("Finished")}</span>}
          <span className="sub"><Crest crest={p.house_crest} color={p.house_color} /> {p.house_name}{p.due_at ? <> · {t("by")} <When iso={p.due_at} /></> : null}</span>
          {editable && <button style={{ marginLeft: "auto" }} onClick={() => put(`/projects/${p.id}`, { state: p.state === "done" ? "open" : "done" })}>{p.state === "done" ? t("Reopen") : "✓ " + t("Mark finished")}</button>}
        </h2>
        {p.notes && <p>{p.notes}</p>}
        <h3 style={{ display: "flex", alignItems: "center", gap: ".6rem" }}>{t("Tasks")} <span className="small">{doneTasks.length} / {tasks.length}</span> <button className="lesser" onClick={() => setAddTask(!addTask)}>+ {t("Add a task")}</button></h3>
        {addTask && (
          <form className="inline" onSubmit={async (e) => { e.preventDefault(); await api(`/projects/${p.id}/tasks`, { method: "POST", body: tf }); setTf({ title: "", due_at: todayIso, notes: "" }); setAddTask(false); load(); }}>
            <div className="row">
              <label>{t("Title")}<input value={tf.title} onChange={(e) => setTf({ ...tf, title: e.target.value })} required maxLength={120} /></label>
              <label>{t("Due")}<input type="date" value={tf.due_at} onChange={(e) => setTf({ ...tf, due_at: e.target.value })} /></label>
              <label>{t("Notes")}<input value={tf.notes} onChange={(e) => setTf({ ...tf, notes: e.target.value })} maxLength={300} /></label>
            </div>
            <div className="submit"><button className="primary" type="submit">{t("Save")}</button></div>
          </form>
        )}
        {tasks.length === 0 && <Empty text={t("No tasks yet. Split the job into pieces a house can take.")} />}
        {openTasks.map((x) => <Task key={x.id} x={x} />)}
        {doneTasks.map((x) => <Task key={x.id} x={x} />)}
        {editable && <p className="small" style={{ marginTop: ".6rem" }}><button className="ghost danger" onClick={() => confirm(p.title + "?") && api(`/projects/${p.id}`, { method: "DELETE" }).then(() => location.assign("#/projects"))}>🗑 {t("Delete project")}</button></p>}
      </div>
      <div className="parchment">
        <h2>🔔 {t("Events")} <button style={{ marginLeft: "auto" }} onClick={() => nav(`/tavern?project=${p.id}`)}>+ {t("Add an event")}</button></h2>
        {events.length === 0 && <Empty text={t("No dates yet.")} />}
        {events.filter((e) => (e.ends_at || e.starts_at) >= today).map((e) => <EventCard key={e.id} ev={e} me={me} reload={load} />)}
        {events.some((e) => (e.ends_at || e.starts_at) < today) && <h3 className="small" style={{ marginTop: ".8rem" }}>{t("Happened")}</h3>}
        {events.filter((e) => (e.ends_at || e.starts_at) < today).map((e) => <EventCard key={e.id} ev={e} me={me} reload={load} />)}
      </div>
    </>
  );
}
