import { useEffect, useState } from "react";
import { useT } from "./i18n";
import { Empty } from "./rooms/shared";

// What changed in the portal in the last 90 days, read off the git log at
// build time (scripts/changelog.mjs writes public/changelog.json). The
// builder's own commit subjects, in English, grouped by day — a panel at the
// foot of Home that answers "is the thing I asked for in yet", without a
// message to the builder.
type Entry = { h: string; d: string; s: string };
type Log = { built: string | null; entries: Entry[] };

export function Changelog() {
  const { t, lang } = useT();
  const [log, setLog] = useState<Log | null>(null);
  const [err, setErr] = useState(false);
  useEffect(() => {
    fetch(`${import.meta.env.BASE_URL}changelog.json`, { cache: "no-cache" })
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then(setLog)
      .catch(() => setErr(true));
  }, []);
  const day = (d: string) => new Date(d + "T00:00").toLocaleDateString(lang === "sl" ? "sl-SI" : "en-GB", { weekday: "long", day: "numeric", month: "long", year: "numeric" });
  const days: [string, Entry[]][] = [];
  for (const e of log?.entries ?? []) {
    const last = days[days.length - 1];
    if (last && last[0] === e.d) last[1].push(e); else days.push([e.d, [e]]);
  }
  return (
    <div className="parchment">
      <h2>🔨 {t("Changelog")} <span className="sub">{t("what changed in the last 90 days, newest first")}</span></h2>
      <p className="small muted">{t("The builder's notes, in English.")}{log?.built && <> · {t("Built from the change of")} {day(log.built)}.</>}</p>
      {err && <Empty text={t("Could not load the changelog.")} />}
      {log && log.entries.length === 0 && <Empty text={t("Nothing in the last 90 days.")} />}
      {days.map(([d, es]) => (
        <section key={d} className="cl-day">
          <h3>{day(d)}</h3>
          <ul>{es.map((e) => <li key={e.h}>{e.s}</li>)}</ul>
        </section>
      ))}
    </div>
  );
}
