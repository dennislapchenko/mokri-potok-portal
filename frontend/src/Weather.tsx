import { useEffect, useState } from "react";
import { api } from "./api";
import { useT } from "./i18n";

// Weather on the home screen. The backend fetches ARSO and trims it, so the
// agency never sees a villager's browser and no third-party frame sits on a
// logged-in page. Attribution stays visible: the data is theirs.

type Day = { date: string; icon: string; text: string; min: string; max: string; rain: string };
type W = { place: string; now: string; now_icon: string; now_text: string; wind: string; days: Day[]; source: string; fetched: string };

// ARSO names its icons in parts: cloud cover, then the weather, then day/night.
function emoji(icon: string): string {
  const i = (icon || "").toLowerCase();
  if (i.includes("ts")) return "⛈️";
  if (i.includes("sn")) return "🌨️";
  if (i.includes("rasn")) return "🌨️";
  if (i.includes("ra") || i.includes("rain") || i.includes("dz")) return "🌧️";
  if (i.includes("fg")) return "🌫️";
  if (i.includes("overcast")) return "☁️";
  if (i.includes("mostcloudy")) return "🌥️";
  if (i.includes("partcloudy")) return "⛅";
  if (i.includes("slightcloudy")) return "🌤️";
  if (i.includes("clear_night")) return "🌙";
  if (i.includes("clear")) return "☀️";
  return "🌡️";
}

export function Weather() {
  const { t, lang } = useT();
  const [w, setW] = useState<W | null>(null);
  const [err, setErr] = useState(false);
  useEffect(() => { api<W>("/weather").then(setW).catch(() => setErr(true)); }, []);

  if (err) return <div className="parchment weather"><p className="small muted">{t("The weather service did not answer.")}</p></div>;
  if (!w) return <div className="parchment weather"><p className="small muted">{t("Loading…")}</p></div>;
  const locale = lang === "sl" ? "sl-SI" : "en-GB";
  // The backend serves the last good copy when ARSO is down, so the panel must
  // say how old the reading is rather than promise it is fresh.
  const fetchedAt = new Date(w.fetched);
  const mins = Math.max(0, Math.round((Date.now() - fetchedAt.getTime()) / 60000));
  const stale = mins > 60;
  const age = mins < 60 ? `${mins} min` : fetchedAt.toLocaleString(locale, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
  const dayName = (d: string, i: number) => (i === 0 ? t("today") : new Date(d + "T00:00").toLocaleDateString(locale, { weekday: "short" }));

  return (
    <div className="parchment weather">
      <div className="w-now">
        <span className="w-icon">{emoji(w.now_icon)}</span>
        <span className="w-temp">{w.now}°</span>
        <span className="w-where">{w.place}<br /><span className="small">{w.now_text}{w.wind ? ` · ${w.wind}` : ""}</span></span>
      </div>
      <div className="w-days">
        {w.days.map((d, i) => (
          <div key={d.date} className="w-day">
            <div className="small">{dayName(d.date, i)}</div>
            <div className="w-icon-sm">{emoji(d.icon)}</div>
            <div className="w-range"><strong>{d.max}°</strong> <span className="muted">{d.min}°</span></div>
            {d.rain && <div className="small w-rain">💧 {d.rain} mm</div>}
          </div>
        ))}
      </div>
      <p className={"small w-src" + (stale ? " stale-note" : " muted")}>
        {t("Vir: ARSO. Forecast for")} {w.place}, {t("not measured at the village.")}{" "}
        {stale ? t("This reading is over an hour old.") : t("Read")} {age}
      </p>
    </div>
  );
}
