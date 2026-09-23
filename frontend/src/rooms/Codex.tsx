import { useState } from "react";
import { api, ApiError, type Me } from "../api";
import { langName, localeOf, useT } from "../i18n";
import { Crest, Empty, parse, useList } from "./shared";

// The Codex: the village's values and agreements, as the council adopted them
// and as the houses have amended them since. Sections are rows in the
// database with one text per language the village speaks; the page shows the
// reader's language and falls back to the first of the village's that has
// text, saying which. Any house edits a section or adds one, a steward
// removes one, and every section names the house that last wrote it — a text
// anyone may change must show who did.

type Text = { title: string; body: string };
type Section = {
  id: number; ord: number; texts: Record<string, Text>;
  updated_at: string; updated_by: number | null; rev: number;
  house_name?: string | null; house_crest?: string | null; house_color?: string | null;
};
// rev rides along on an edit so the backend can tell the form opened on a
// section that has since moved under it.
type Form = { texts: Record<string, Text>; rev?: number };
const blank: Form = { texts: {} };
const empty: Text = { title: "", body: "" };

const ROMAN = ["I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII", "XIII", "XIV", "XV", "XVI", "XVII", "XVIII", "XIX", "XX"];

export function Codex({ me }: { me: Me }) {
  const { t, lang, languages } = useT();
  const { items, reload } = useList<Section>("/codex");
  // The language a section is shown in: the reader's if it has text, else the
  // first of the village's that does.
  const shown = (s: Section) => [lang, ...languages].find((l) => s.texts[l]?.title || s.texts[l]?.body) || lang;
  const [editing, setEditing] = useState<number | "new" | null>(null);
  const [f, setF] = useState<Form>(blank);
  // The section as another house left it while this form was open — shown
  // above the form after a 409, so the writer can read it without losing
  // their own text. The next save then carries the newer stamp, knowingly.
  const [conflict, setConflict] = useState<Section | null>(null);
  const start = (s?: Section) => {
    setF(s ? { texts: { ...s.texts }, rev: s.rev } : blank);
    setEditing(s ? s.id : "new"); setConflict(null);
  };
  const setText = (l: string, patch: Partial<Text>) => setF({ ...f, texts: { ...f.texts, [l]: { ...(f.texts[l] || empty), ...patch } } });
  const hasTitle = Object.values(f.texts).some((x) => x.title);
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
  const day = (s: string) => parse(s).toLocaleDateString(localeOf(lang), { day: "numeric", month: "long", year: "numeric" });
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
          <h3>{conflict.texts[shown(conflict)]?.title}</h3>
          <Prose text={conflict.texts[shown(conflict)]?.body || ""} />
          <p className="small muted">{t("Your text is below, untouched. Saving it replaces theirs.")}</p>
        </div>
      )}
      {/* One title and one text per language the village speaks, in its order. */}
      {languages.map((l) => (
        <div key={l}>
          <label>{t("Title")} ({langName(l)})<input value={f.texts[l]?.title || ""} onChange={(e) => setText(l, { title: e.target.value })} maxLength={120} /></label>
          <label>{t("Text")} ({langName(l)})<textarea value={f.texts[l]?.body || ""} onChange={(e) => setText(l, { body: e.target.value })} rows={8} maxLength={8000} /></label>
        </div>
      ))}
      <p className="small muted">{t("A blank line starts a new paragraph. A line that starts with \"- \" is a bullet, and the words before its first colon are set in bold.")}</p>
      <div className="submit">
        <button type="button" className="ghost" onClick={() => { setEditing(null); setConflict(null); }}>✕</button>
        <button className="primary" type="submit" disabled={!hasTitle}>{t("Save")}</button>
      </div>
    </form>
  );

  return (
    <div className="parchment codex">
      <h2>📜 {t("Codex")} <span className="sub">{t("the village's values and agreements")}</span></h2>
      {last && <p className="cx-stamp cx-head"><Stamp s={last} /></p>}
      {items.length === 0 && <Empty text={t("The codex is empty. A steward brings in the adopted text.")} />}
      {items.map((s, i) => {
        const l = shown(s);
        const text = s.texts[l] || empty;
        return (
          <section key={s.id} className="cx-sec">
            <div className="cx-num">{ROMAN[i] || i + 1}.</div>
            <div className="cx-text">
              {editing === s.id ? form : (<>
                <h3>{text.title}</h3>
                {l !== lang && <p className="small muted" style={{ fontStyle: "italic" }}>{t("Not translated yet — language shown:")} {langName(l)}.</p>}
                <Prose text={text.body} />
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
