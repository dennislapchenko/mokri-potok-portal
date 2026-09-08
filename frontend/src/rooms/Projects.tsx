import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { EventCard } from "./EventCard";
import { Crest, Empty, When, canEdit, useList } from "./shared";
import { DatePicker } from "../DatePicker";
import { photoURL, uploadPhoto } from "../photo";

// Projects: a long job split into tasks. A task is taken, never handed to a
// house. Done is a state that stays visible. "3 of 5" is a project's progress,
// never a house's score.

export function Projects({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList("/projects");
  const [open, setOpen] = useState(false);
  const [f, setF] = useState({ title: "", due_at: "", notes: "" });
  const nav = useNavigate();

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    const { id } = await api<{ id: number }>("/projects", { method: "POST", body: f });
    setF({ title: "", due_at: "", notes: "" }); setOpen(false); reload();
    nav(`/projects/${id}`);
  };
  const live = items.filter((p) => p.state === "open"), done = items.filter((p) => p.state === "done");
  // Render functions, not components — see Market.tsx. Lowercase on purpose.
  const card = (p: any) => (
    <Link key={p.id} to={`/projects/${p.id}`} className="card project" style={{ opacity: p.state === "done" ? 0.7 : 1 }}>
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
            <label>{t("Due")}<DatePicker value={f.due_at} onChange={(v) => setF({ ...f, due_at: v })} placeholder={t("no due date")} /></label>
          </div>
          <label>{t("Notes")}<textarea value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} maxLength={2000} /></label>
          <div className="submit"><button className="primary" type="submit">{t("Create")}</button></div>
        </form>
      )}
      {items.length === 0 && <Empty text={t("No projects yet. Start one — a fence, a roof, a road.")} />}
      {live.map(card)}
      {done.length > 0 && <details style={{ marginTop: "1rem" }}><summary className="small">✓ {t("Finished")} ({done.length})</summary>{done.map(card)}</details>}
    </div>
  );
}

export function Project({ me, houses: allHouses }: { me: Me; houses: { id: number; name: string; crest: string; kind?: string }[] }) {
  const houses = allHouses.filter((h) => h.kind !== "common"); // land does not take tasks
  const { t } = useT();
  const { id } = useParams();
  const nav = useNavigate();
  const [p, setP] = useState<any>(null);
  const [tf, setTf] = useState({ title: "", due_at: "", notes: "" });
  const [addTask, setAddTask] = useState(false);
  const [closing, setClosing] = useState<number | null>(null);
  const [note, setNote] = useState("");
  const [editing, setEditing] = useState(false);
  const [pf, setPf] = useState({ title: "", due_at: "", notes: "" });
  const [editTask, setEditTask] = useState<number | null>(null);
  const [ef, setEf] = useState({ title: "", due_at: "", notes: "" });
  const load = () => api<any>(`/projects/${id}`).then(setP).catch(() => setP(null));
  useEffect(() => { load(); }, [id]); // eslint-disable-line react-hooks/exhaustive-deps
  if (!p) return <div className="parchment">{t("Loading…")}</div>;

  const editable = canEdit(me, p);
  const tasks: any[] = p.tasks || [], events: any[] = p.events || [];
  const openTasks = tasks.filter((x) => x.state === "open"), doneTasks = tasks.filter((x) => x.state === "done");
  const today = new Date().toISOString().slice(0, 10);
  const put = (path: string, body: any) => api(path, { method: "PUT", body }).then(load);
  const startEdit = () => { setPf({ title: p.title, due_at: p.due_at || "", notes: p.notes || "" }); setEditing(true); };
  const startEditTask = (x: any) => { setEf({ title: x.title, due_at: x.due_at || "", notes: x.notes || "" }); setEditTask(x.id); };

  // A render function, not a component — see Market.tsx. Lowercase on purpose.
  const task = (x: any) => {
    const mine = x.assigned_to === me.id;
    const creator = x.house_id === me.id || editable;
    return (
      <div key={x.id} className="card" style={{ opacity: x.state === "done" ? 0.7 : 1, borderLeftColor: x.state === "done" ? "var(--parch3)" : x.assigned_to ? "var(--brass)" : "var(--green)" }}>
        {editTask === x.id ? (
          <form className="inline" onSubmit={(e) => { e.preventDefault(); put(`/tasks/${x.id}`, ef).then(() => setEditTask(null)); }}>
            <div className="row">
              <label>{t("Title")}<input value={ef.title} onChange={(e) => setEf({ ...ef, title: e.target.value })} required maxLength={120} autoFocus /></label>
              <label>{t("Due")}<DatePicker value={ef.due_at} onChange={(v) => setEf({ ...ef, due_at: v })} placeholder={t("no due date")} /></label>
              <label>{t("Notes")}<input value={ef.notes} onChange={(e) => setEf({ ...ef, notes: e.target.value })} maxLength={300} /></label>
            </div>
            <div className="submit"><button type="button" className="ghost" onClick={() => setEditTask(null)}>✕</button><button className="primary" type="submit">{t("Save")}</button></div>
          </form>
        ) : (<>
        <div className="head">
          <strong>{x.state === "done" ? "✓ " : ""}{x.title}</strong>
          {x.assigned_to ? <span className="tag taken">{x.assigned_crest} {x.assigned_name}</span> : x.state === "open" ? <span className="tag open">{t("free to take")}</span> : null}
          {x.due_at && <span className="when">{t("by")} <When iso={x.due_at} /></span>}
        </div>
        {(x.notes || x.closing_note) && <div className="body small">{x.notes}{x.closing_note && <div>📝 {x.closing_note}</div>}</div>}
        </>)}
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
          {creator && editTask !== x.id && <button className="ghost" onClick={() => startEditTask(x)}>✎ {t("Edit")}</button>}
          {creator && <button className="ghost" onClick={() => confirm(x.title + "?") && api(`/tasks/${x.id}`, { method: "DELETE" }).then(load)}>🗑</button>}
        </div>
      </div>
    );
  };

  return (
    <>
      <div className="parchment">
        <p className="small"><Link to="/projects">← {t("Projects")}</Link></p>
        {editing ? (
          <form className="inline" onSubmit={(e) => { e.preventDefault(); put(`/projects/${p.id}`, pf).then(() => setEditing(false)); }}>
            <div className="row">
              <label>{t("Title")}<input value={pf.title} onChange={(e) => setPf({ ...pf, title: e.target.value })} required maxLength={120} autoFocus /></label>
              <label>{t("Due")}<DatePicker value={pf.due_at} onChange={(v) => setPf({ ...pf, due_at: v })} placeholder={t("no due date")} /></label>
            </div>
            <label>{t("Notes")}<textarea value={pf.notes} onChange={(e) => setPf({ ...pf, notes: e.target.value })} maxLength={2000} /></label>
            <div className="submit"><button type="button" className="ghost" onClick={() => setEditing(false)}>✕</button><button className="primary" type="submit">{t("Save")}</button></div>
          </form>
        ) : (<>
        <h2>📋 {p.title} {p.state === "done" && <span className="tag done">✓ {t("Finished")}</span>}
          <span className="sub"><Crest crest={p.house_crest} color={p.house_color} /> {p.house_name}{p.due_at ? <> · {t("by")} <When iso={p.due_at} /></> : null}</span>
          {editable && <span style={{ marginLeft: "auto", display: "flex", gap: ".4rem" }}>
            <button className="ghost" onClick={startEdit}>✎ {t("Edit")}</button>
            <button onClick={() => put(`/projects/${p.id}`, { state: p.state === "done" ? "open" : "done" })}>{p.state === "done" ? t("Reopen") : "✓ " + t("Mark finished")}</button>
          </span>}
        </h2>
        {p.notes && <p>{p.notes}</p>}
        </>)}
        <h3 style={{ display: "flex", alignItems: "center", gap: ".6rem" }}>{t("Tasks")} <span className="small">{doneTasks.length} / {tasks.length}</span> <button className="lesser" onClick={() => setAddTask(!addTask)}>+ {t("Add a task")}</button></h3>
        {addTask && (
          <form className="inline" onSubmit={async (e) => { e.preventDefault(); await api(`/projects/${p.id}/tasks`, { method: "POST", body: tf }); setTf({ title: "", due_at: "", notes: "" }); setAddTask(false); load(); }}>
            <div className="row">
              <label>{t("Title")}<input value={tf.title} onChange={(e) => setTf({ ...tf, title: e.target.value })} required maxLength={120} /></label>
              <label>{t("Due")}<DatePicker value={tf.due_at} onChange={(v) => setTf({ ...tf, due_at: v })} placeholder={t("no due date")} /></label>
              <label>{t("Notes")}<input value={tf.notes} onChange={(e) => setTf({ ...tf, notes: e.target.value })} maxLength={300} /></label>
            </div>
            <div className="submit"><button className="primary" type="submit">{t("Save")}</button></div>
          </form>
        )}
        {tasks.length === 0 && <Empty text={t("No tasks yet. Split the job into pieces a house can take.")} />}
        {openTasks.map(task)}
        {doneTasks.map(task)}
        {editable && <p className="small" style={{ marginTop: ".6rem" }}><button className="ghost danger" onClick={() => confirm(p.title + "?") && api(`/projects/${p.id}`, { method: "DELETE" }).then(() => location.assign("#/projects"))}>🗑 {t("Delete project")}</button></p>}
      </div>
      <Pictures p={p} me={me} reload={load} />
      <div className="parchment">
        <h2>🔔 {t("Events")} <button style={{ marginLeft: "auto" }} onClick={() => nav(`/tavern?project=${p.id}`)}>+ {t("Add an event")}</button></h2>
        {events.length === 0 && <Empty text={t("No dates yet.")} />}
        {events.filter((e) => (e.ends_at || e.starts_at) >= today).map((e) => <EventCard key={e.id} ev={e} me={me} reload={load} linkToTavern />)}
        {events.some((e) => (e.ends_at || e.starts_at) < today) && <h3 className="small" style={{ marginTop: ".8rem" }}>{t("Happened")}</h3>}
        {events.filter((e) => (e.ends_at || e.starts_at) < today).map((e) => <EventCard key={e.id} ev={e} me={me} reload={load} linkToTavern />)}
      </div>
    </>
  );
}

// Pictures: a thin strip of square thumbnails, above the events. Any house adds
// one — a project belongs to the village the way its events do — and the
// full-size view says which house and when. The house that added it, the
// project's house or a steward can take it down. The file input opens the
// chooser, not the camera: before-and-after pictures are already in the gallery.
function Pictures({ p, me, reload }: { p: any; me: Me; reload: () => void }) {
  const { t } = useT();
  const [big, setBig] = useState<{ url: string; f: any } | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const photos: any[] = p.photos || [];
  const add = async (fl: File | null) => {
    if (!fl) return;
    setBusy(true); setErr("");
    try { await uploadPhoto(`/projects/${p.id}/photos`, fl, "POST"); reload(); } catch { setErr(t("The picture did not upload. Try again.")); } finally { setBusy(false); }
  };
  const mayDelete = (f: any) => f.house_id === me.id || p.house_id === me.id || me.is_steward === 1;
  return (
    <div className="parchment">
      <h2>📷 {t("Pictures")} <span className="sub">{t("before, during, after")}</span>
        <label className="btn-file" style={{ marginLeft: "auto", opacity: busy ? 0.6 : 1 }}>+ {t("Add a picture")}<input type="file" accept="image/*" disabled={busy} onChange={(e) => { add(e.target.files?.[0] || null); e.target.value = ""; }} /></label>
      </h2>
      {err && <p className="small" style={{ color: "var(--red)" }}>{err}</p>}
      {photos.length === 0 && <Empty text={t("No pictures yet.")} />}
      {photos.length > 0 && <div className="thumbs">{photos.map((f) => <Thumb key={f.id} f={f} onOpen={(url) => setBig({ url, f })} onDelete={mayDelete(f) ? () => confirm("?") && api(`/photos/${f.id}`, { method: "DELETE" }).then(reload) : undefined} />)}</div>}
      {big && <div className="lightbox" onClick={() => setBig(null)}><img src={big.url} alt="" /><div className="cap">{big.f.house_name ? big.f.house_name + " · " : ""}<When iso={big.f.created_at} /></div></div>}
    </div>
  );
}

function Thumb({ f, onOpen, onDelete }: { f: any; onOpen: (url: string) => void; onDelete?: () => void }) {
  const [url, setUrl] = useState("");
  useEffect(() => { photoURL(`/photos/${f.id}`).then(setUrl).catch(() => setUrl("")); }, [f.id]);
  return (
    <figure className="thumb" title={f.house_name || ""}>
      {url && <img src={url} alt="" onClick={() => onOpen(url)} />}
      {onDelete && <button type="button" className="ghost thumb-del" onClick={onDelete}>🗑</button>}
    </figure>
  );
}
