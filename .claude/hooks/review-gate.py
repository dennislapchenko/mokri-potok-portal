#!/usr/bin/env python3
"""Stop hook: the turn may not end with edits unreviewed.

Reads the Stop event from stdin, walks this turn's slice of the transcript
(everything after the last real user message), and blocks once if files were
written and the `reviewer` subagent was not run. `stop_hook_active` is true when
the turn is already continuing because of this hook, so the block fires at most
once per turn and can never loop.

A write is a file tool, or a Bash command that looks like one: a redirect, a
heredoc, `sed -i`, `tee`, `mv`, `cp`, `rm`, `git commit`. That pattern list is
the honest limit: a script that writes from inside python or go is not seen,
and a `>` inside a quoted string is a false positive that costs one reminder.
"""
import json
import re
import sys

WRITE_TOOLS = {"Edit", "Write", "MultiEdit", "NotebookEdit"}
BASH_WRITE = re.compile(
    r"(?<![<>|])>{1,2}(?!&)|<<-?\s*['\"]?\w|\bsed\s+(-\w*\s+)*-i\b|\btee\b|\bmv\b|\bcp\b|\brm\b|\bgit\s+commit\b"
)


def main() -> int:
    try:
        event = json.load(sys.stdin)
    except Exception:
        return 0
    if event.get("stop_hook_active"):
        return 0
    path = event.get("transcript_path")
    if not path:
        return 0
    try:
        with open(path, encoding="utf-8") as f:
            lines = [json.loads(l) for l in f if l.strip()]
    except Exception:
        return 0

    # A real user message has text content, a tool result is also type=user.
    start = 0
    for i, rec in enumerate(lines):
        if rec.get("type") != "user":
            continue
        content = (rec.get("message") or {}).get("content")
        if isinstance(content, str):
            start = i
        elif isinstance(content, list) and not any(
            isinstance(b, dict) and b.get("type") == "tool_result" for b in content
        ):
            start = i

    wrote, reviewed = False, False
    for rec in lines[start:]:
        if rec.get("type") != "assistant":
            continue
        for block in (rec.get("message") or {}).get("content") or []:
            if not isinstance(block, dict) or block.get("type") != "tool_use":
                continue
            name = block.get("name", "")
            inp = block.get("input") or {}
            if name in WRITE_TOOLS:
                wrote = True
            if name == "Bash" and BASH_WRITE.search(inp.get("command", "")):
                wrote = True
            if name == "Agent" and inp.get("subagent_type") == "reviewer":
                reviewed = True

    if wrote and not reviewed:
        sys.stderr.write(
            "Files were changed this turn and the reviewer has not seen them. "
            "Invoke the `reviewer` subagent (.claude/agents/reviewer.md) with "
            "what the turn was asked to do and what was done, then act on its "
            "feedback or surface it to the owner before ending the turn.\n"
        )
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
