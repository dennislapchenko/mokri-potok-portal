import { useEffect, useMemo, useRef, useState } from "react";
import type { House } from "../api";
import { useT } from "../i18n";

// Cadastral parcels in EPSG:3794 metres, drawn straight into an SVG with the
// northing flipped. No tiles, no projection library: the village is ~1 km wide.
type Feature = { properties: { parcel: string; area_m2: number; e: number; n: number }; geometry: { type: string; coordinates: any } };
type Parcels = { features: Feature[] };
type Channels = { segments: number[][] };

// EPSG:3794 metres. The collective's plot is the village's high ground; the
// event ground lies north-west across the stream. Stewards refine by assigning.
const VILLAGE_CENTER = { e: 486450, n: 46480 };

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
  const [channels, setChannels] = useState<Channels | null>(null);
  const [tip, setTip] = useState<string>("");
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    const base = import.meta.env.BASE_URL;
    fetch(base + "data/parcels.geojson").then((r) => r.json()).then(setParcels).catch(() => setParcels({ features: [] }));
    fetch(base + "data/channels.json").then((r) => r.json()).then(setChannels).catch(() => setChannels(null));
  }, []);

  const owner = useMemo(() => {
    const m = new Map<string, House>();
    for (const h of houses) for (const p of h.parcels || []) m.set(p, h);
    return m;
  }, [houses]);

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
    // Nothing assigned yet: open on the village centre, not the whole 2.5 km box.
    if (!own.length || !isFinite(minE)) return { x: VILLAGE_CENTER.e - 350, y: -VILLAGE_CENTER.n - 350, w: 700, h: 700 };
    const pad = own.length ? 60 : 0;
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

  if (!parcels || !view) return <div className="map-wrap"><div style={{ padding: "2rem", textAlign: "center" }}>{t("Loading…")}</div></div>;

  const strokeW = view.w / 900;
  const fontPx = view.w / 60;
  return (
    <div className="map-wrap">
      <svg ref={svgRef} viewBox={`${view.x} ${view.y} ${view.w} ${view.h}`} onPointerDown={onDown} onPointerMove={onMove} onPointerUp={onUp} onPointerLeave={onUp} onWheel={onWheel}>
        {parcels.features.map((f) => {
          const p = f.properties.parcel;
          const h = owner.get(p);
          const sel = selected?.includes(p);
          const fill = sel ? "#e0c072" : h ? h.color : "#e9dcb8";
          const op = h ? (highlight && h.id !== highlight ? 0.45 : 0.85) : 0.6;
          return ringsOf(f).map((ring, i) => (
            <path key={p + i}
              d={ring.map(([e, n], j) => (j ? "L" : "M") + e + " " + -n).join(" ") + "Z"}
              fill={fill} fillOpacity={op} stroke={sel ? "#8a2f2f" : "#6b5a44"} strokeWidth={sel ? strokeW * 2.5 : strokeW}
              style={{ cursor: onParcelClick ? "pointer" : "grab" }}
              onClick={() => onParcelClick?.(p)}
              onPointerEnter={() => setTip(h ? `${h.crest} ${h.name} · ${p}` : onParcelClick ? p : "")}
              onPointerLeave={() => setTip("")}
            />
          ));
        })}
        {channels?.segments.map((s, i) => (
          <line key={i} x1={s[0]} y1={-s[1]} x2={s[2]} y2={-s[3]} stroke="#3f5f8a" strokeWidth={strokeW * 1.6} strokeDasharray={`${strokeW * 4} ${strokeW * 3}`} strokeOpacity={0.7} />
        ))}
        {houses.filter((h) => h.parcels?.length).map((h) => {
          // Crest at the centroid of the house's first parcel.
          const f = parcels.features.find((x) => h.parcels.includes(x.properties.parcel));
          if (!f) return null;
          return (
            <text key={h.id} x={f.properties.e} y={-f.properties.n} fontSize={fontPx} textAnchor="middle" dominantBaseline="central" style={{ pointerEvents: "none" }}>
              {h.crest}
            </text>
          );
        })}
      </svg>
      <div className="map-controls">
        <button aria-label={t("zoom in")} onClick={() => zoom(1 / 1.4)}>+</button>
        <button aria-label={t("zoom out")} onClick={() => zoom(1.4)}>−</button>
        <button aria-label={t("reset")} onClick={() => setView(home)}>⌂</button>
      </div>
      {tip && <div className="map-tip">{tip}</div>}
      <div className="map-note">{t("Cadastre snapshot 2026-08-15 (GURS). Lines are legal boundaries, not fences. Dashed blue: modelled water paths near the plot, not a survey.")}</div>
    </div>
  );
}
