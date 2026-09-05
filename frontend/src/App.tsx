import { useCallback, useEffect, useState } from "react";
import { Link, NavLink, Route, Routes, useNavigate, useParams } from "react-router-dom";
import { api, ApiError, getToken, setToken, type House, type Me } from "./api";
import { syncPushLang } from "./push";
import { useT } from "./i18n";
import { VillageMap } from "./map/VillageMap";
import { Hall } from "./rooms/Hall";
import { Market } from "./rooms/Market";
import { Watchtower } from "./rooms/Watchtower";
import { Houses } from "./rooms/Houses";
import { ToolShed } from "./rooms/ToolShed";
import { Projects, Project } from "./rooms/Projects";
import { Camp } from "./rooms/Camp";
import { InstallBanner } from "./Install";

export type Session = { me: Me; houses: House[]; refresh: () => Promise<void>; logout: () => void };

// `short` is the bottom-nav label: a phone gives each item about 60 px, and
// "Lopa z orodjem" does not fit. `nav: false` keeps a room off the bottom bar —
// Houses is reached by tapping your own house name in the top bar.
export const ROOMS = [
  { path: "/tavern", icon: "🍺", name: "Tavern", short: "Hall", sub: "calendar, work bees and the board", nav: true },
  { path: "/projects", icon: "📋", name: "Projects", short: "Projects", sub: "long jobs, split into tasks", nav: false },
  { path: "/market", icon: "🧺", name: "Market", short: "Market", sub: "needs, give-aways, runs", nav: true },
  { path: "/shed", icon: "🛠", name: "Tool shed", short: "Shed", sub: "what the village lends", nav: true },
  { path: "/watch", icon: "🕯️", name: "Watchtower", short: "Watch", sub: "who is away", nav: true },
  { path: "/camp", icon: "🏕️", name: "Campground", short: "Camp", sub: "who collected from whom", nav: false },
  { path: "/houses", icon: "🏘️", name: "Houses", short: "Houses", sub: "houses and land", nav: false },
];

export default function App() {
  const { t, lang, setLang } = useT();
  const [me, setMe] = useState<Me | null>(null);
  const [houses, setHouses] = useState<House[]>([]);
  const [state, setState] = useState<"loading" | "gate" | "in">(getToken() ? "loading" : "gate");

  const refresh = useCallback(async () => {
    const [m, hs] = await Promise.all([api<Me>("/me"), api<House[]>("/houses")]);
    setMe(m); setHouses(hs); setState("in");
    syncPushLang();
  }, []);

  useEffect(() => {
    if (!getToken()) return;
    refresh().catch((e) => { if (e instanceof ApiError && e.status === 401) setToken(null); setState("gate"); });
  }, [refresh]);

  const logout = () => { setToken(null); setMe(null); setState("gate"); };

  return (
    <>
      <header className="topbar">
        <div className="inner">
          <h1><Link to="/">🏰 Mokri Potok</Link> <span className="small" style={{ color: "var(--parch2)" }}>· {t("Village portal")}</span></h1>
          {me && <Link className="house" to="/houses"><span className="crest" style={{ background: me.color }}>{me.crest}</span> {me.name}</Link>}
          <span className="lang">
            <button className={lang === "sl" ? "primary" : ""} onClick={() => setLang("sl")}>SL</button>{" "}
            <button className={lang === "en" ? "primary" : ""} onClick={() => setLang("en")}>EN</button>
          </span>
        </div>
      </header>
      <main>
        {state === "loading" && <div className="parchment">{t("Loading…")}</div>}
        {state === "gate" && (
          <div className="gate-bg" style={{ "--bg": `url(${import.meta.env.BASE_URL}backdrop.jpg)` } as React.CSSProperties}>
            <Routes>
              <Route path="/join/:code" element={<Gate onJoined={refresh} />} />
              <Route path="*" element={<Gate onJoined={refresh} />} />
            </Routes>
          </div>
        )}
        {state === "in" && me && (
          <Routes>
            <Route path="/" element={<Home me={me} houses={houses} />} />
            <Route path="/tavern" element={<Hall me={me} />} />
            {/* Notifications sent before the merge point at #/bell — keep it alive. */}
            <Route path="/bell" element={<Hall me={me} />} />
            <Route path="/market" element={<Market me={me} houses={houses} />} />
            <Route path="/watch" element={<Watchtower me={me} />} />
            <Route path="/shed" element={<ToolShed me={me} houses={houses} />} />
            <Route path="/projects" element={<Projects me={me} />} />
            <Route path="/projects/:id" element={<Project me={me} houses={houses} />} />
            <Route path="/camp" element={<Camp me={me} />} />
            <Route path="/houses" element={<Houses me={me} houses={houses} refresh={refresh} logout={logout} />} />
            <Route path="/join/:code" element={<Home me={me} houses={houses} />} />
            <Route path="*" element={<Home me={me} houses={houses} />} />
          </Routes>
        )}
      </main>
      {state === "in" && (
        <nav className="bottomnav">
          <NavLink to="/" end><span className="icon">🗺️</span>{t("Village")}</NavLink>
          {ROOMS.filter((r) => r.nav).map((r) => <NavLink key={r.path} to={r.path}><span className="icon">{r.icon}</span>{t(r.short)}</NavLink>)}
        </nav>
      )}
    </>
  );
}

function Home({ me, houses }: { me: Me; houses: House[] }) {
  const { t } = useT();
  // One badge per building, built as a finished sentence so a room can say two
  // things at once (the shed: how many are in it, how many are out).
  const [badges, setBadges] = useState<Record<string, string>>({});
  useEffect(() => {
    Promise.all([api<any[]>("/needs"), api<any[]>("/events"), api<any[]>("/away"), api<any[]>("/posts"), api<any[]>("/tools"), api<any[]>("/projects"), api<any[]>("/camp")])
      .then(([n, e, a, p, tools, projects, camp]) => {
        // Local date, not UTC: between midnight and 02:00 CEST a UTC date
        // counts a finished event as still ahead.
        const n2 = (v: number) => String(v).padStart(2, "0");
        const d = new Date();
        const today = `${d.getFullYear()}-${n2(d.getMonth() + 1)}-${n2(d.getDate())}`;
        const num = (v: number, label: string) => (v > 0 ? `${v} ${t(label)}` : "");
        const out = tools.filter((x) => x.held_by).length;
        setBadges({
          "/market": num(n.filter((x) => x.state === "open").length, "open needs"),
          "/watch": num(a.filter((x) => x.from_date <= today && x.to_date >= today).length, "away now"),
          "/tavern": [num(e.filter((x) => (x.ends_at || x.starts_at) >= today).length, "events ahead"), num(p.filter((x) => x.pinned).length, "pinned")].filter(Boolean).join(" · "),
          "/shed": [num(tools.length - out, "in the shed"), num(out, "out")].filter(Boolean).join(" · "),
          "/projects": [num(projects.filter((x) => x.state === "open").length, "open"), num(projects.reduce((acc, x) => acc + (x.state === "open" ? x.tasks_free : 0), 0), "tasks free to take")].filter(Boolean).join(" · "),
          "/camp": num(camp.filter((x) => x.state === "held").length, "held"),
        });
      }).catch(() => {});
  }, [t]);
  return (
    <>
      <InstallBanner />
      <VillageMap houses={houses} highlight={me.id} />
      <div className="legend">
        {houses.map((h) => <span key={h.id}><span className="swatch" style={{ background: h.color }} /> {h.crest} {h.name}{h.id === me.id ? ` (${t("Your house")})` : ""}</span>)}
      </div>
      <div className="buildings">
        {ROOMS.map((r) => (
          <Link key={r.path} to={r.path} className="building">
            <span className="icon">{r.icon}</span>
            <span className="sign">{t(r.name)}</span>
            <span className="small">{t(r.sub)}</span>
            {badges[r.path] && <span className="badge">{badges[r.path]}</span>}
          </Link>
        ))}
      </div>
    </>
  );
}

// Gate: join with an invite code (from the link or typed), or found the village.
function Gate({ onJoined }: { onJoined: () => Promise<void> }) {
  const { t } = useT();
  const { code: urlCode } = useParams();
  const nav = useNavigate();
  const [code, setCode] = useState(urlCode || "");
  const [device, setDevice] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState("");
  const [bootstrap, setBootstrap] = useState(false);
  useEffect(() => { api<any>("/status").then((s) => setBootstrap(!!s.bootstrap_needed)).catch(() => {}); }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault(); setErr("");
    try {
      const path = bootstrap ? "/bootstrap" : "/join";
      const r = await api<{ token: string }>(path, { method: "POST", body: { code: code.trim(), device, name } });
      setToken(r.token);
      await onJoined();
      nav("/");
    } catch (ex) {
      const s = ex instanceof ApiError ? ex.status : 0;
      setErr(s === 404 || s === 403 ? t("Unknown code") : s === 410 ? t("Code expired — ask the steward for a new link") : s === 429 ? t("Too many attempts. Wait a few minutes.") : t("Something went wrong"));
    }
  };
  return (
    <div className="parchment gate">
      <h2>🏰 {t("Enter the village")}</h2>
      <p className="small">{bootstrap ? t("The village is empty. The first steward founds it with the bootstrap code from the server log.") : t("You need an invite link from a steward. Ask in the WhatsApp group.")}</p>
      {!bootstrap && <p className="small">{t("Already signed in in your browser? Open the portal there, go to Houses and press Add another phone — it gives you a 6-digit code for this one.")}</p>}
      <form onSubmit={submit} className="inline">
        <label>{bootstrap ? t("Bootstrap code") : t("Invite or pairing code")}<input value={code} onChange={(e) => setCode(e.target.value)} autoCapitalize="off" autoComplete="off" required /></label>
        {bootstrap && <label>{t("Steward house name")}<input value={name} onChange={(e) => setName(e.target.value)} /></label>}
        <label>{t("This phone")}<input value={device} onChange={(e) => setDevice(e.target.value)} placeholder={t("e.g. Ana's phone")} /></label>
        {err && <div className="err">{err}</div>}
        <div className="submit"><button className="primary" type="submit">{bootstrap ? t("Found the village") : t("Join")}</button></div>
      </form>
    </div>
  );
}
