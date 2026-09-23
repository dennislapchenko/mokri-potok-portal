import { useEffect, useMemo, useRef, useState } from "react";
import { api, ApiError, type House } from "../api";
import { useT } from "../i18n";
import { Water } from "./Water";

// Cadastral parcels in planar metres (this village: EPSG:3794), drawn straight
// into an SVG with the northing flipped. No tiles, no projection library: a
// village is ~1 km wide. The data is a row in the database, entered once with
// `/server map-import`; a village without it sees what it needs instead.
type Feature = { properties: { parcel: string; area_m2: number; e: number; n: number }; geometry: { type: string; coordinates: any } };
type Parcels = { features: Feature[] };
type MapMeta = { name: string; source: string; snapshot: string };

function ringsOf(f: Feature): number[][][] {
  const g = f.geometry;
  if (g.type === "Polygon") return g.coordinates as number[][][];
  if (g.type === "MultiPolygon") return (g.coordinates as number[][][][]).flat();
  return [];
}

export function VillageMap({ houses, selected, onParcelClick, highlight }: {
  houses: House[];
  selected?: string[];          // parcels being edited (steward assign mode)
  onParcelClick?: (parcel: string) => void;
  highlight?: number;           // house id to emphasise
}) {
  const { t } = useT();
  const [parcels, setParcels] = useState<Parcels | null>(null);
  const [missing, setMissing] = useState(false); // no parcels row: the village has no map yet
  const [failed, setFailed] = useState(false);   // anything else: say so, never spin forever
  const [meta, setMeta] = useState<MapMeta | null>(null);
  const [tip, setTip] = useState<string>("");
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    api<Parcels>("/map/parcels").then(setParcels).catch((e) => { if (e instanceof ApiError && e.status === 404) setMissing(true); else setFailed(true); });
    api<MapMeta[]>("/map").then((rows) => setMeta(rows.find((m) => m.name === "parcels") || null)).catch(() => {});
  }, []);

  const owner = useMemo(() => {
    const m = new Map<string, House>();
    for (const h of houses) for (const p of h.parcels || []) m.set(p, h);
    return m;
  }, [houses]);

  // Where the crests go. A house sits on the first parcel it holds; a house
  // that lives on somebody else's land sits on that land too, beside the crest
  // of the house that holds it. Nothing here changes who owns what — the
  // parcel keeps the holder's colour and the holder's parcel list.
  const marks = useMemo(() => {
    const at = new Map<string, { h: House; guest: boolean }[]>();
    const add = (p: string, h: House, guest: boolean) => at.set(p, [...(at.get(p) || []), { h, guest }]);
    const placed = new Set<number>();
    for (const f of parcels?.features || []) {
      const p = f.properties.parcel;
      const o = owner.get(p);
      if (o && !placed.has(o.id)) {
        placed.add(o.id);
        add(p, o, false);
      }
      for (const g of houses) if (g.homes?.includes(p)) add(p, g, true);
    }
    return at;
  }, [parcels, houses, owner]);

  const featureOf = useMemo(() => {
    const m = new Map<string, Feature>();
    for (const f of parcels?.features || []) m.set(f.properties.parcel, f);
    return m;
  }, [parcels]);

  // Default view: the collective's parcels if any are assigned, else the
  // 500 m around the middle of the data.
  const home = useMemo(() => {
    if (!parcels) return null;
    const own = parcels.features.filter((f) => owner.has(f.properties.parcel));
    const pool = own.length ? own : parcels.features;
    let minE = Infinity, maxE = -Infinity, minN = Infinity, maxN = -Infinity;
    for (const f of pool) for (const ring of ringsOf(f)) for (const [e, n] of ring) {
      if (e < minE) minE = e; if (e > maxE) maxE = e; if (n < minN) minN = n; if (n > maxN) maxN = n;
    }
    if (!isFinite(minE)) return null;
    // Nothing assigned yet: open 700 m around the middle of the data, not the
    // whole box. The binary knows no village, so no fixed centre lives here.
    if (!own.length) return { x: (minE + maxE) / 2 - 350, y: -(minN + maxN) / 2 - 350, w: 700, h: 700 };
    const pad = 60;
    const w = Math.max(maxE - minE + 2 * pad, 250), h = Math.max(maxN - minN + 2 * pad, 250);
    const cx = (minE + maxE) / 2, cy = -(minN + maxN) / 2;
    const s = Math.max(w, h);
    return { x: cx - s / 2, y: cy - s / 2, w: s, h: s };
  }, [parcels, owner]);

  const [view, setView] = useState<{ x: number; y: number; w: number; h: number } | null>(null);
  useEffect(() => { if (home && !view) setView(home); }, [home, view]);

  const zoom = (f: number) => setView((v) => v && { x: v.x + (v.w - v.w * f) / 2, y: v.y + (v.h - v.h * f) / 2, w: v.w * f, h: v.h * f });

  // Drag to pan (pointer events; touch-action none on the svg).
  const drag = useRef<{ x: number; y: number; vx: number; vy: number } | null>(null);
  const onDown = (e: React.PointerEvent) => { if (view) drag.current = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y }; };
  const onMove = (e: React.PointerEvent) => {
    if (!drag.current || !view || !svgRef.current) return;
    const rect = svgRef.current.getBoundingClientRect();
    const k = view.w / rect.width;
    setView({ ...view, x: drag.current.vx - (e.clientX - drag.current.x) * k, y: drag.current.vy - (e.clientY - drag.current.y) * k });
  };
  const onUp = () => { drag.current = null; };
  const onWheel = (e: React.WheelEvent) => { e.preventDefault(); zoom(e.deltaY > 0 ? 1.15 : 1 / 1.15); };

  if (missing || failed) {
    // Every villager reads the first line; the how is for the steward, who
    // has the README and the server. The others have a phone.
    const steward = houses.some((h) => h.id === highlight && h.is_steward);
    return (
      <div className="map-wrap map-empty">
        <p><strong>{t(missing ? "This village has no map yet." : "The map did not load.")}</strong></p>
        {missing && steward && <p className="small">{t("A steward imports the cadastre on the server, as a GeoJSON FeatureCollection in planar metres — the README says how.")}</p>}
      </div>
    );
  }
  if (!parcels || !view) return <div className="map-wrap"><div style={{ padding: "2rem", textAlign: "center" }}>{t("Loading…")}</div></div>;

  const strokeW = view.w / 900;
  const fontPx = view.w / 60;
  const tipFor = (p: string) => {
    const h = owner.get(p);
    const guests = houses.filter((g) => g.homes?.includes(p)).map((g) => `${g.crest} ${g.name} (${t("lives here")})`);
    const who = [h ? `${h.crest} ${h.name}` : "", ...guests].filter(Boolean).join(" · ");
    return who ? `${who} · ${p}` : onParcelClick ? p : "";
  };
  return (
    <div className="map-wrap">
      <svg ref={svgRef} viewBox={`${view.x} ${view.y} ${view.w} ${view.h}`} onPointerDown={onDown} onPointerMove={onMove} onPointerUp={onUp} onPointerLeave={onUp} onWheel={onWheel}>
        {parcels.features.map((f) => {
          const p = f.properties.parcel;
          const h = owner.get(p);
          const sel = selected?.includes(p);
          const fill = sel ? "#e0c072" : h ? h.color : "#e9dcb8";
          const op = h ? (highlight && h.id !== highlight ? 0.45 : 0.85) : 0.6;
          // A parcel nobody holds and nobody lives on is context, not content:
          // its outline goes half-strength so the village reads out of the
          // cadastre mesh at a glance. Selected and lived-on keep full lines.
          const claimed = !!h || sel || marks.has(p);
          return ringsOf(f).map((ring, i) => (
            <path key={p + i}
              d={ring.map(([e, n], j) => (j ? "L" : "M") + e + " " + -n).join(" ") + "Z"}
              fill={fill} fillOpacity={op} stroke={sel ? "#8a2f2f" : "#6b5a44"} strokeWidth={sel ? strokeW * 2.5 : strokeW} strokeOpacity={claimed ? 1 : 0.5}
              style={{ cursor: onParcelClick ? "pointer" : "grab" }}
              onClick={() => onParcelClick?.(p)}
              onPointerEnter={() => setTip(tipFor(p))}
              onPointerLeave={() => setTip("")}
            />
          ));
        })}
        <Water scale={strokeW} />
        {[...marks].map(([p, list]) => {
          const f = featureOf.get(p);
          if (!f) return null;
          // Several crests on one parcel spread sideways so neither hides the other.
          return list.map((m, i) => (
            <text key={p + "-" + m.h.id} x={f.properties.e + (i - (list.length - 1) / 2) * fontPx * 0.95} y={-f.properties.n}
              fontSize={m.guest ? fontPx * 0.8 : fontPx} textAnchor="middle" dominantBaseline="central" style={{ pointerEvents: "none" }}>
              {m.h.crest}
            </text>
          ));
        })}
      </svg>
      <div className="map-controls">
        <button aria-label={t("zoom in")} onClick={() => zoom(1 / 1.4)}>+</button>
        <button aria-label={t("zoom out")} onClick={() => zoom(1.4)}>−</button>
        <button aria-label={t("reset")} onClick={() => setView(home)}>⌂</button>
      </div>
      {tip && <div className="map-tip">{tip}</div>}
      {meta && meta.snapshot && <div className="map-note">{t("Cadastre snapshot")} {meta.snapshot} ({meta.source}).</div>}
    </div>
  );
}
