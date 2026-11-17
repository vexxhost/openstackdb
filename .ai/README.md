# .ai

Tool-agnostic workflows for AI agents working on this repository. Project rules live in `/AGENTS.md`; this directory holds task-specific playbooks.

| File | Use it to |
| --- | --- |
| `context/security.md` | Security policy every agent follows while working here |
| `security-review.md` | Run a security review of the branch's changes, or a baseline review of the whole repository, and record the results |
| `security-review-false-positives.md` | Verify or reject candidate findings from that review |

Write these files as plain Markdown instructions, with no tool-specific syntax. Wrappers for a particular agent (Claude Code skills in `.claude/skills/`, Cursor rules, Copilot prompts, …) should reference these files instead of copying them, so every tool follows the same rules.
