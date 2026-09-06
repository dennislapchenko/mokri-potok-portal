import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import type { Me } from "../api";
import { useT } from "../i18n";
import { Calendar } from "./Calendar";
import { Board } from "./Board";
import { Crest, When, useList } from "./shared";

// The tavern is one room: pinned notices on the door, the calendar on the wall,
// the board under it. It was two rooms until 2026-09-04 — a village opens one
// door, and the bottom bar only fits so many.
export function Hall({ me, houses }: { me: Me; houses: any[] }) {
  const { t } = useT();
  const posts = useList("/posts");
  const [params] = useSearchParams();

  // A post notification links to ?at=board — jump past the calendar.
  useEffect(() => {
    if (params.get("at") === "board") document.getElementById("board")?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [params, posts.items.length]);

  const pinned = posts.items.filter((p: any) => p.pinned && !p.parent_id);
  return (
    <>
      {pinned.length > 0 && (
        <div className="parchment pinned-strip">
          <h2>📌 {t("Pinned")}</h2>
          {pinned.map((p: any) => (
            <div key={p.id} className="card pinned">
              <div className="head"><Crest crest={p.house_crest} color={p.house_color} /><span className="who">{p.house_name}</span><When iso={p.created_at} /></div>
              <div className="body">{p.body}</div>
            </div>
          ))}
        </div>
      )}
      <Calendar me={me} houses={houses} />
      <div id="board">
        <Board me={me} items={posts.items} reload={posts.reload} />
      </div>
    </>
  );
}
