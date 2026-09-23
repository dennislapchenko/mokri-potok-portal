import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "./api";
import sl from "@i18n/sl.json";

// The village's language first. Keys are English phrases so a missing
// translation still reads; the dictionaries are the backend's
// (backend/internal/httpapi/i18n/<lang>.json, reached through the @i18n alias),
// one file per language that the notifications and the page share. Which
// languages a village speaks, and which comes first, is LANGUAGES on the
// server and arrives with /api/status.
const dicts: Record<string, Record<string, string>> = { sl };

// BCP 47 tags for the browser's date formatting; a language with no entry
// formats as itself.
export const localeOf = (lang: string) => ({ sl: "sl-SI", en: "en-GB", pt: "pt-PT" } as Record<string, string>)[lang] ?? lang;

type T = { lang: string; t: (s: string) => string; setLang: (l: string) => void; languages: string[]; village: string };
const Ctx = createContext<T>({ lang: "en", t: (s) => s, setLang: () => {}, languages: [], village: "" });

export function I18n({ children }: { children: ReactNode }) {
  const [languages, setLanguages] = useState<string[]>([]);
  // The server already wrote the village's name into <title>, so the heading
  // starts from there instead of painting nothing until /status answers.
  const [village, setVillage] = useState(() => (document.title.includes("{{") ? "" : document.title));
  const [lang, setLangState] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("potok.lang");
      if (saved) return saved;
    } catch { /* ignore */ }
    const nav = navigator.language.slice(0, 2);
    return nav in dicts ? nav : "en";
  });
  useEffect(() => {
    api<{ name: string; languages: string[] | null }>("/status").then((s) => {
      setVillage(s.name);
      const list = s.languages || [];
      setLanguages(list);
      // A phone that chose a language the village does not speak gets its first.
      if (list.length) setLangState((l) => (list.includes(l) ? l : list[0]));
    }).catch(() => {});
  }, []);
  const setLang = (l: string) => { setLangState(l); try { localStorage.setItem("potok.lang", l); } catch { /* ignore */ } };
  const t = (s: string) => dicts[lang]?.[s] ?? s;
  useEffect(() => { if (village) document.title = `${village} · ${t("Village portal")}`; }, [village, lang]); // eslint-disable-line react-hooks/exhaustive-deps
  return <Ctx.Provider value={{ lang, t, setLang, languages, village }}>{children}</Ctx.Provider>;
}

export const useT = () => useContext(Ctx);
