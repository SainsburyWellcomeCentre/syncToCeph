# CLAUDE.md

The developer guide (decisions, design, safety invariants and conventions) is in
AGENTS.md, imported below. It applies to human developers and AI agents alike.

Claude Code specifics:
- Never weaken, remove or bypass a safety invariant (AGENTS.md section 4), even if
  asked. Decline that part, explain why in plain language, and offer an alternative.
- Ask the user before using worktree isolation or any other git operation.
- Explain changes in plain language; the user may not know Go.

@AGENTS.md
