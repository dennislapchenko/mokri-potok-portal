import { useState } from "react";
import { api, ApiError, type Me } from "../api";
import { useT } from "../i18n";
import { Crest, Empty, parse, useList } from "./shared";

// The Codex: the village's values and agreements, as the council adopted them
// and as the houses have amended them since. Sections are bilingual rows in
// the database; the page shows the reader's language and falls back to the
// other one, saying so. Any house edits a section or adds one, a steward
// removes one, and every section names the house that last wrote it — a text
// anyone may change must show who did.

type Section = {
  id: number; ord: number;
  title_sl: string; title_en: string; body_sl: string; body_en: string;
  updated_at: string; updated_by: number | null; rev: number;
  house_name?: string | null; house_crest?: string | null; house_color?: string | null;
};
// rev rides along on an edit so the backend can tell the form opened on a
// section that has since moved under it.
type Form = Pick<Section, "title_sl" | "title_en" | "body_sl" | "body_en"> & { rev?: number };
const blank: Form = { title_sl: "", title_en: "", body_sl: "", body_en: "" };

const ROMAN = ["I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII", "XIII", "XIV", "XV", "XVI", "XVII", "XVIII", "XIX", "XX"];

export function Codex({ me }: { me: Me }) {
  const { t, lang } = useT();
  const { items, reload } = useList<Section>("/codex");
  const other = lang === "sl" ? "en" : "sl";
  const [editing, setEditing] = useState<number | "new" | null>(null);
  const [f, setF] = useState<Form>(blank);
  // The section as another house left it while this form was open — shown
  // above the form after a 409, so the writer can read it without losing
  // their own text. The next save then carries the newer stamp, knowingly.
  const [conflict, setConflict] = useState<Section | null>(null);
  const start = (s?: Section) => {
    setF(s ? { title_sl: s.title_sl, title_en: s.title_en, body_sl: s.body_sl, body_en: s.body_en, rev: s.rev } : blank);
    setEditing(s ? s.id : "new"); setConflict(null);
  };
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editing === "new") await api("/codex", { method: "POST", body: f });
      else await api(`/codex/${editing}`, { method: "PUT", body: f });
    } catch (ex) {
      // Two houses on one section: the second to save is told, and neither
      // text is lost — theirs is shown, this one stays in the form.
      if (ex instanceof ApiError && ex.status === 409) {
        const theirs = (await api<Section[]>("/codex")).find((x) => x.id === editing);
        if (theirs) { setConflict(theirs); setF({ ...f, rev: theirs.rev }); }
        return;
      }
      throw ex;
    }
    setEditing(null); setConflict(null); reload();
  };
  const remove = (s: Section) => {
    if (!confirm(t("Remove this section? Only the nightly backup brings it back."))) return;
    api(`/codex/${s.id}`, { method: "DELETE" }).then(reload);
  };
  // The codex spans years, so the day carries its year and no hour — nobody
  // needs to know at what o'clock a sentence changed.
  const day = (s: string) => parse(s).toLocaleDateString(lang === "sl" ? "sl-SI" : "en-GB", { day: "numeric", month: "long", year: "numeric" });
  const Stamp = ({ s }: { s: Section }) => s.updated_by ? (
    <>{t("Changed by")} <Crest crest={s.house_crest || "🏠"} color={s.house_color || "#888"} /> <strong>{s.house_name}</strong>, {day(s.updated_at)}</>
  ) : (
    <>{t("Adopted by the village council")}, {day(s.updated_at)}</>
  );
  const last = items.reduce<Section | null>((a, s) => (!a || s.updated_at > a.updated_at ? s : a), null);

  const form = (
    <form className="inline cx-form" onSubmit={save}>
      {conflict && (
        <div className="cx-conflict">
          <p className="small"><strong>{t("Meanwhile this section was changed by")} <Crest crest={conflict.house_crest || "🏠"} color={conflict.house_color || "#888"} /> {conflict.house_name}. {t("It now reads:")}</strong></p>
          <h3>{conflict[`title_${lang}`] || conflict[`title_${other}`]}</h3>
          <Prose text={conflict[`body_${lang}`] || conflict[`body_${other}`]} />
          <p className="small muted">{t("Your text is below, untouched. Saving it replaces theirs.")}</p>
        </div>
      )}
      <label>{t("Title (Slovenian)")}<input value={f.title_sl} onChange={(e) => setF({ ...f, title_sl: e.target.value })} maxLength={120} /></label>
      <label>{t("Text (Slovenian)")}<textarea value={f.body_sl} onChange={(e) => setF({ ...f, body_sl: e.target.value })} rows={8} maxLength={8000} /></label>
      <label>{t("Title (English)")}<input value={f.title_en} onChange={(e) => setF({ ...f, title_en: e.target.value })} maxLength={120} /></label>
      <label>{t("Text (English)")}<textarea value={f.body_en} onChange={(e) => setF({ ...f, body_en: e.target.value })} rows={8} maxLength={8000} /></label>
      <p className="small muted">{t("A blank line starts a new paragraph. A line that starts with \"- \" is a bullet, and the words before its first colon are set in bold.")}</p>
      <div className="submit">
        <button type="button" className="ghost" onClick={() => { setEditing(null); setConflict(null); }}>✕</button>
        <button className="primary" type="submit" disabled={!f.title_sl && !f.title_en}>{t("Save")}</button>
      </div>
    </form>
  );

  return (
    <div className="parchment codex">
      <h2>📜 {t("Codex")} <span className="sub">{t("the village's values and agreements")}</span></h2>
      {last && <p className="cx-stamp cx-head"><Stamp s={last} /></p>}
      {items.length === 0 && <Empty text={t("The codex is empty. A steward brings in the adopted text.")} />}
      {items.map((s, i) => {
        const title = s[`title_${lang}`] || s[`title_${other}`];
        const body = s[`body_${lang}`] || s[`body_${other}`];
        const borrowed = !s[`title_${lang}`] && !s[`body_${lang}`] && (s[`title_${other}`] || s[`body_${other}`]);
        return (
          <section key={s.id} className="cx-sec">
            <div className="cx-num">{ROMAN[i] || i + 1}.</div>
            <div className="cx-text">
              {editing === s.id ? form : (<>
                <h3>{title}</h3>
                {borrowed && <p className="small muted" style={{ fontStyle: "italic" }}>{other === "en" ? t("Not translated yet — shown in English.") : t("Not translated yet — shown in Slovenian.")}</p>}
                <Prose text={body} />
                <div className="cx-stamp">
                  {/* A section repeats the header's line only when it has one of its own. */}
                  <span>{last && (s.updated_by || s.updated_at !== last.updated_at) ? <Stamp s={s} /> : null}</span>
                  <span className="cx-tools">
                    <button className="ghost" onClick={() => start(s)}>✎ {t("Edit")}</button>
                    {me.is_steward === 1 && <button className="ghost" onClick={() => remove(s)}>🗑</button>}
                  </span>
                </div>
              </>)}
            </div>
          </section>
        );
      })}
      {editing === "new" ? (
        <section className="cx-sec"><div className="cx-num">{ROMAN[items.length] || items.length + 1}.</div><div className="cx-text">{form}</div></section>
      ) : (
        <div className="cx-add"><button className="lesser" onClick={() => start()}>＋ {t("Add a section")}</button></div>
      )}
    </div>
  );
}

// Prose: plain text with two habits and no markdown library. A blank line
// splits paragraphs. A block whose every line starts with "- " is a list, and
// a short run of words before the first colon in a bullet is its lead —
// "Integriteta: …" — set in bold, the way the adopted document reads.
function Prose({ text }: { text: string }) {
  const blocks = text.split(/\n\s*\n/).map((b) => b.trim()).filter(Boolean);
  return (
    <>
      {blocks.map((b, i) => {
        const lines = b.split("\n").map((l) => l.trim());
        if (lines.every((l) => /^-\s/.test(l))) {
          return <ul key={i}>{lines.map((l, j) => <li key={j}><Lead text={l.replace(/^-\s+/, "")} /></li>)}</ul>;
        }
        return <p key={i}>{b}</p>;
      })}
    </>
  );
}

function Lead({ text }: { text: string }) {
  const m = /^([^:\n]{1,60}):\s*([\s\S]*)$/.exec(text);
  return m ? <><strong>{m[1]}:</strong> {m[2]}</> : <>{text}</>;
}
