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

// The tag for the browser's date formatting. English is pinned to en-GB —
// day before month, as the village reads it — a bare "en" would give US
// order; every other language formats under its own tag.
export const localeOf = (lang: string) => (lang === "en" ? "en-GB" : lang);
// A language's own name for itself — the one word that is never translated.
// The browser knows it; a browser that does not shows the tag.
export const langName = (lang: string) => { try { return new Intl.DisplayNames([lang], { type: "language" }).of(lang) || lang; } catch { return lang; } };

type T = { lang: string; t: (s: string) => string; setLang: (l: string) => void; languages: string[]; village: string };
const Ctx = createContext<T>({ lang: "en", t: (s) => s, setLang: () => {}, languages: [], village: "" });

// The server writes the village's name into <title> and its language list
// into a meta tag, so both are known before anything is fetched. The Vite dev
// server serves the raw tokens; then /status fills them in.
const served = (s: string | null | undefined) => (s && !s.includes("{{") ? s : "");
const metaLanguages = () => served(document.querySelector('meta[name="languages"]')?.getAttribute("content")).split(",").filter(Boolean);

export function I18n({ children }: { children: ReactNode }) {
  const [languages, setLanguages] = useState<string[]>(metaLanguages);
  const [village, setVillage] = useState(() => served(document.title));
  const [lang, setLangState] = useState<string>(() => {
    const list = metaLanguages();
    try {
      const saved = localStorage.getItem("potok.lang");
      if (saved && (!list.length || list.includes(saved))) return saved;
    } catch { /* ignore */ }
    if (list.length) {
      // The phone's own language if the village speaks it, else the village's first.
      const nav = navigator.language.slice(0, 2);
      return list.includes(nav) ? nav : list[0];
    }
    const nav = navigator.language.slice(0, 2);
    return nav in dicts ? nav : "en";
  });
  useEffect(() => {
    if (languages.length && village) return; // served by the binary, nothing to fetch
    api<{ name: string; languages: string[] | null }>("/status").then((s) => {
      setVillage(s.name);
      const list = s.languages || [];
      setLanguages(list);
      if (list.length) setLangState((l) => (list.includes(l) ? l : list[0]));
    }).catch(() => {});
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const setLang = (l: string) => { setLangState(l); try { localStorage.setItem("potok.lang", l); } catch { /* ignore */ } };
  const t = (s: string) => dicts[lang]?.[s] ?? s;
  useEffect(() => { if (village) document.title = `${village} · ${t("Village portal")}`; }, [village, lang]); // eslint-disable-line react-hooks/exhaustive-deps
  return <Ctx.Provider value={{ lang, t, setLang, languages, village }}>{children}</Ctx.Provider>;
}

export const useT = () => useContext(Ctx);
