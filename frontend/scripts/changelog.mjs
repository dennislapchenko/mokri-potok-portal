// Writes public/changelog.json from the git log, so the page can show what
// changed and when. Runs before every build and dev start. Where there is no
// git — the Docker web stage copies frontend/ without .git — it keeps the file
// CI wrote a moment earlier, or writes an empty one so the page still loads.
//
// Left out: deploy roll commits, docs-only and tooling commits, merges. The
// commit subjects here are written for the villager who reads them.
import { execFileSync } from "node:child_process";
import { existsSync, writeFileSync } from "node:fs";

const out = new URL("../public/changelog.json", import.meta.url);
const skip = /^(deploy|docs|ci|chore|tests?|refactor|taskfile)\b|^revert\b|\[skip ci\]|^merge\b/i;

try {
  // A window, not the whole history: the page sits at the foot of Home and a
  // list with no end is nobody's answer. Older lines are in git.
  const git = (...args) => execFileSync("git", args, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
  const built = git("log", "-1", "--date=short", "--format=%ad") || null;
  const raw = git("log", "--since=90 days ago", "--date=short", "--format=%h%x1f%ad%x1f%s");
  const all = raw.split("\n").filter(Boolean).map((l) => { const [h, d, s] = l.split("\x1f"); return { h, d, s }; });
  const entries = all.filter((e) => !skip.test(e.s));
  writeFileSync(out, JSON.stringify({ built, entries }));
  console.log(`changelog: ${entries.length} of ${all.length} commits`);
} catch {
  if (existsSync(out)) console.log("changelog: no git here, keeping the one already written");
  else { writeFileSync(out, JSON.stringify({ built: null, entries: [] })); console.log("changelog: no git here, wrote an empty one"); }
}
