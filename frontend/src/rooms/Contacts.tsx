import { useEffect, useMemo, useRef, useState } from "react";
import { api, type Me } from "../api";
import { useT } from "../i18n";
import { Empty, canEdit, useList } from "./shared";
import { Thread } from "./Thread";

// Contacts: the village's phone book — the well-driller, the vet, the man with
// the plough. The chat has every one of these numbers, once, somewhere above.
//
// Any house writes a number down and any house corrects it, the same footing
// as editing an event, and the card names the house that last wrote it. The
// thread under a number is for whether it still works and who came last — the
// tradesman is not in the room to answer, so it is not a place to rate one.
// Nothing here rings a phone (owner's decision 2026-09-10): a phone book is
// looked up when the pipe bursts, not announced.

type Contact = {
  id: number; house_id: number | null; name: string; phone: string; notes: string; type: string;
  edited_by_name: string | null; comments: number;
};

const blank = { name: "", phone: "", notes: "", type: "" };

export function Contacts({ me }: { me: Me }) {
  const { t } = useT();
  const { items, reload } = useList<Contact>("/contacts");
  const [f, setF] = useState(blank);
  const [q, setQ] = useState("");
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<number | null>(null);
  const [ef, setEf] = useState(blank);
  const [openThread, setOpenThread] = useState<number | null>(null);

  // The types the book already carries. There is no table of types: a type
  // exists exactly as long as a contact wears it.
  const types = useMemo(() => [...new Set(items.map((c) => c.type).filter(Boolean))].sort((a, b) => a.localeCompare(b)), [items]);
  // One box over both fields, because a villager looking for the vet may
  // remember the trade and not the name, or the name and not the trade. The
  // order is left alone: a phone book that reshuffles as you type is harder
  // to read than one that only gets shorter.
  const shown = useMemo(() => {
    const needle = q.trim();
    if (!needle) return items;
    return items.filter((c) => fuzzy(needle, c.name) !== null || (c.type && fuzzy(needle, c.type) !== null));
  }, [items, q]);

  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    await api("/contacts", { method: "POST", body: f });
    setF(blank); setAdding(false); reload();
  };
  const saveEdit = async (id: number) => {
    await api(`/contacts/${id}`, { method: "PUT", body: ef });
    setEditing(null); reload();
  };
  const startEdit = (c: Contact) => { setEf({ name: c.name, phone: c.phone, notes: c.notes, type: c.type }); setEditing(c.id); };
  const del = (c: Contact) => confirm(c.name + " — " + t("the number and everything written under it")) && api(`/contacts/${c.id}`, { method: "DELETE" }).then(reload);

  // The form is the same three fields whether it adds or corrects.
  const fields = (v: typeof blank, set: (x: typeof blank) => void) => (<>
    <div className="row">
      <label>{t("Name")}<input value={v.name} onChange={(e) => set({ ...v, name: e.target.value })} required maxLength={80} placeholder={t("e.g. the well-driller, the vet")} /></label>
      <label>{t("Phone")}<input value={v.phone} onChange={(e) => set({ ...v, phone: e.target.value })} maxLength={40} inputMode="tel" placeholder="041 000 000" /></label>
    </div>
    <label>{t("Type")}<TypePicker value={v.type} onChange={(x) => set({ ...v, type: x })} types={types} /></label>
    <label>{t("Notes")}<textarea value={v.notes} onChange={(e) => set({ ...v, notes: e.target.value })} maxLength={300} placeholder={t("speaks German, comes on Tuesdays")} /></label>
  </>);

  return (
    <div className="parchment">
      <h2>📇 {t("Contacts")} <span className="sub">{t("who to call")}</span></h2>
      <p><button onClick={() => setAdding(!adding)}>+ {t("Add")}</button></p>
      {adding && <form className="inline" onSubmit={add}>{fields(f, setF)}
        <div className="submit"><button className="primary" type="submit">📇 {t("Write it down")}</button></div>
      </form>}
      {items.length > 1 && <input className="find" value={q} onChange={(e) => setQ(e.target.value)} placeholder={"🔍 " + t("a name or a type")} aria-label={t("a name or a type")} />}
      {items.length === 0 && <Empty text={t("No numbers yet. Write the first one down — the chat will lose it.")} />}
      {items.length > 0 && shown.length === 0 && <Empty text={t("No number matches that.")} />}
      {shown.map((c) => (
        <div key={c.id} className="card contact">
          {editing === c.id ? (
            <form className="inline" onSubmit={(e) => { e.preventDefault(); saveEdit(c.id); }}>{fields(ef, setEf)}
              <div className="submit"><button type="button" className="ghost" onClick={() => setEditing(null)}>✕</button><button className="primary" type="submit">{t("Save")}</button></div>
            </form>
          ) : (<>
            <div className="head">
              <strong>{c.name}</strong>
              {c.type && <span className="tag">{c.type}</span>}
              {/* A number on a phone is there to be pressed. */}
              {c.phone && <a className="btn primary tel" href={"tel:" + c.phone.replace(/[^+0-9]/g, "")}>☎ {c.phone}</a>}
            </div>
            {c.notes && <div className="body small">{c.notes}</div>}
            {/* Who wrote a number down is not worth a line — a number is the
                village's, not a house's. Who changed one is, because anyone
                may, and a text anyone may change shows who did. */}
            {c.edited_by_name && <div className="small muted">✎ {t("last edited by")} {c.edited_by_name}</div>}
          </>)}
          <div className="actions">
            <button className="ghost" onClick={() => setOpenThread(openThread === c.id ? null : c.id)}>💬 {t("Comments")}{c.comments ? ` (${c.comments})` : ""}</button>
            {editing !== c.id && <button className="ghost" onClick={() => startEdit(c)}>✎ {t("Edit")}</button>}
            {canEdit(me, c) && <button className="ghost" onClick={() => del(c)}>🗑 {t("Delete")}</button>}
          </div>
          {openThread === c.id && <Thread subject="contact" id={c.id} me={me} onChanged={reload} />}
        </div>
      ))}
    </div>
  );
}

// fuzzy: fzf's idea, small enough to keep. The letters you typed must appear
// in order but need not be adjacent, and the score prefers a run of them and
// a letter that begins a word — so "vt" finds "veterinar" over "vodovodar
// traktorist". null means no match at all. The last term breaks ties towards
// the shorter word, which is the one you were more likely reaching for.
//
// Both sides lose their accents first: a villager typing fast, or on a phone
// keyboard set to English, writes "cebelar" and means Čebelar. Slovenian is
// the language of this room, so it may not be the one that fails to match.
const plain = (v: string) => v.toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "");

export function fuzzy(q: string, s: string): number | null {
  const a = plain(q), b = plain(s);
  let i = 0, score = 0, prev = -2;
  for (let j = 0; j < b.length && i < a.length; j++) {
    if (b[j] !== a[i]) continue;
    score += j === prev + 1 ? 3 : 1;
    if (j === 0 || /[\s\-/,.]/.test(b[j - 1])) score += 2;
    prev = j; i++;
  }
  return i === a.length ? score - b.length * 0.01 : null;
}

// TypePicker: one control, not two. The field is a plain text input — whatever
// stands in it is the type — and under it the types the book already carries,
// narrowed as you type the way fzf narrows. Writing a word nobody has used is
// how a new type is made: there is no second control for that, and no list to
// maintain. The backend folds a spelling that differs only in case into the
// one already in use, so "Vet" typed over "vet" does not split the group.
function TypePicker({ value, onChange, types }: { value: string; onChange: (v: string) => void; types: string[] }) {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const [hi, setHi] = useState(0);
  const wrap = useRef<HTMLSpanElement>(null);
  const q = value.trim();

  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => { if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open]);

  const hits = useMemo(() => {
    if (!q) return types;
    return types.map((x) => [x, fuzzy(q, x)] as const).filter((p): p is readonly [string, number] => p[1] !== null)
      .sort((a, b) => b[1] - a[1]).map(([x]) => x);
  }, [types, q]);
  const known = types.some((x) => x.toLowerCase() === q.toLowerCase());
  // Nothing is highlighted until the villager highlights it. A row that is
  // merely the best match must never be taken by an Enter meant for the word
  // just typed — that is the whole point of a picker that also makes types.
  useEffect(() => { setHi(-1); }, [q]);

  const pick = (x: string) => { onChange(x); setOpen(false); };
  const key = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") { setOpen(false); return; }
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault(); setOpen(true);
      const n = hits.length, down = e.key === "ArrowDown";
      if (n) setHi((h) => (down ? (h < 0 ? 0 : (h + 1) % n) : h <= 0 ? n - 1 : h - 1));
      return;
    }
    // Enter takes the row under the highlight, and otherwise closes the list
    // and leaves what was typed standing as a new type. Either way it does not
    // submit the form from an open picker; a second Enter does, as usual.
    if (e.key === "Enter" && open) {
      e.preventDefault();
      if (hi >= 0 && hits[hi]) pick(hits[hi]); else setOpen(false);
    }
  };

  return (
    <span className="dp tp" ref={wrap}>
      <input value={value} onChange={(e) => { onChange(e.target.value); setOpen(true); }} onFocus={() => setOpen(true)} onKeyDown={key}
        maxLength={40} autoComplete="off" placeholder={t("vet, craftsman, office…")} />
      {open && (hits.length > 0 || (q !== "" && !known)) && (
        <div className="dp-pop tp-pop" onClick={(e) => e.preventDefault()}>
          {hits.map((x, i) => (
            <button type="button" key={x} className={"tp-hit" + (i === hi ? " on" : "")} onMouseEnter={() => setHi(i)} onClick={() => pick(x)}>{x}</button>
          ))}
          {q !== "" && !known && <div className="small muted tp-new">＋ {t("new type")}: <strong>{q}</strong></div>}
        </div>
      )}
    </span>
  );
}
