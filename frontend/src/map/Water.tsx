import { useEffect, useState } from "react";
import { useT } from "../i18n";

// Water on the village map. The lines are the watercourses a terrain model
// finds in the LiDAR over the whole village — drawn as water rather than as
// dashes nobody could read: solid blue, the big courses wider, and named along
// their own course the way a paper map names a brook.
//
// Only a course above `stream_m2` is named. The rest are drawn thin and left
// unnamed on purpose: 1 m bare earth says how much land drains into a channel,
// never whether it runs, so calling every small one a gully would be a claim
// the data cannot make.
//
// It is one file, one data file and one line in VillageMap. To take the water
// off the map, delete this file, `public/data/water.json` and the <Water/>
// line, and drop the note about it from the map's caption.
type Line = { flow_m2: number; kind: string; points: [number, number][] };
type Doc = { lines: Line[] };

export function Water({ scale }: { scale: number }) {
  const { t } = useT();
  const [doc, setDoc] = useState<Doc | null>(null);
  useEffect(() => {
    fetch(import.meta.env.BASE_URL + "data/water.json").then((r) => r.json()).then(setDoc).catch(() => {});
  }, []);
  if (!doc) return null;
  // The name is sized in ground metres, capped: it is a whisper when the whole
  // village is on screen and readable as soon as anybody zooms into the water.
  const font = Math.min(7, scale * 22.5);
  // A course now runs for kilometres, so one name at its middle is a name
  // nobody ever has on screen. Repeat it about once per screen width instead.
  const gap = Math.max(150, scale * 900 * 0.9);
  return (
    // The water is drawn, never tapped: a parcel underneath must stay
    // reachable in assign mode.
    <g style={{ pointerEvents: "none" }}>
      {doc.lines.map((l, i) => {
        const stream = l.kind === "stream";
        // Wide enough to read as water when zoomed in, never thinner than a
        // hair when the whole village is on screen.
        const w = stream ? Math.max(scale * 3, 3.2) : Math.max(scale * 1.7, 1.6);
        // The name follows the line, so a course drawn right-to-left would come
        // out upside down. Reading the same line the other way costs nothing.
        const pts = l.points[l.points.length - 1][0] < l.points[0][0] ? [...l.points].reverse() : l.points;
        const d = pts.map(([e, n], j) => (j ? "L" : "M") + e + " " + -n).join(" ");
        let len = 0;
        for (let j = 1; j < pts.length; j++) len += Math.hypot(pts[j][0] - pts[j - 1][0], pts[j][1] - pts[j - 1][1]);
        const n = stream ? Math.min(24, Math.max(1, Math.round(len / gap))) : 0;
        return (
          <g key={i}>
            <path id={`water${i}`} d={d} fill="none" stroke="#4b86b4" strokeOpacity={0.85} strokeWidth={w} strokeLinecap="round" strokeLinejoin="round" />
            {Array.from({ length: n }, (_, k) => (
              <text key={k} fontSize={font} fill="#2f6690" dy={-w} letterSpacing={font * 0.08}>
                <textPath href={`#water${i}`} xlinkHref={`#water${i}`} startOffset={`${((k + 0.5) / n) * 100}%`}>{t("stream")}</textPath>
              </text>
            ))}
          </g>
        );
      })}
    </g>
  );
}
