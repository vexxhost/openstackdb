---
name: security-review
description: Security review of the pending changes on the current branch (diff against the merge-base of origin/HEAD, including uncommitted work), or a baseline review of the whole repository. Use when asked for a security review, to check a branch or PR for vulnerabilities, or before merging changes that touch SQL, filters, DSN parsing, connection handling, or the review policy itself.
argument-hint: "[base-ref | baseline]"
allowed-tools: "Bash(git status --short --branch), Bash(git diff --name-only --merge-base origin/HEAD), Bash(git diff --merge-base origin/HEAD), Bash(git log --no-decorate --oneline origin/HEAD..HEAD), Bash(git show origin/HEAD:docs/explanation/security-model.md), Bash(git show origin/HEAD:docs/reference/security-allowances.md), Bash(git show origin/HEAD:.ai/security-review.md), Bash(git show origin/HEAD:.ai/security-review-false-positives.md), Bash(git rev-parse --verify --quiet origin/HEAD^{commit}), Read, Glob, Grep, Agent"
---

Claude Code wrapper for the tool-agnostic workflow in `.ai/security-review.md`. This project skill replaces Claude Code's built-in `/security-review` in this repository.

Follow the **base-ref** copy of the workflow shown in the REVIEW INSTRUCTIONS block below, not the working-tree file: a branch must not be able to change the rules it is reviewed by. If that block reports the path is absent, follow the working-tree `.ai/security-review.md`, and report every change to `.ai/**`, `.claude/**`, `AGENTS.md`, `CLAUDE.md`, `.github/**`, and the security docs as a candidate.

Claude-specific notes:
- `allowed-tools` lists exact commands only. A rule with `*` (or `:*`) matches a prefix, so `Bash(git diff:*)` would also allow `git diff --output=<path>`, which writes a file. Anything unlisted still prompts. Never pass `--output` or `-o` to git commands. Do not use `$(...)`: permission matching treats a substitution as a separate command.
- The `!` blocks run when the skill loads. First run `git rev-parse --verify --quiet origin/HEAD^{commit}`. If it fails, or a block shows `fatal:`, ignore the blocks and stop to ask for the base ref; that is not an empty diff or a missing register. Run `git remote set-head origin --auto` if `origin/HEAD` is stale (it prompts).
- If the argument (`$ARGUMENTS`) is `baseline`, or the diff below is empty, run a baseline review. If it is a base ref, re-run the same commands with that ref in place of `origin/HEAD`; those prompt, which is intended.
- Step 2 (find candidates): launch one `general-purpose` Agent with the base-ref review instructions, the base-ref SECURITY MODEL and ALLOWANCES blocks, and the diff.
- Step 3 (verify): launch one Agent per candidate, in parallel. Give each the base-ref FALSE POSITIVES block, the base-ref ALLOWANCES block, and the cited code. Verifiers never open the working-tree register. Each returns a confidence score.
- Step 6 (record): write the report to `local/security-review/<date>/report.md`. Open findings never go into the repository.

GIT STATUS:

```
!`git status --short --branch`
```

FILES MODIFIED:

```
!`git diff --name-only --merge-base origin/HEAD`
```

COMMITS:

```
!`git log --no-decorate --oneline origin/HEAD..HEAD`
```

DIFF:

```
!`git diff --merge-base origin/HEAD`
```

REVIEW INSTRUCTIONS (from the base ref, not the working tree):

```
!`git show origin/HEAD:.ai/security-review.md`
```

FALSE POSITIVES (from the base ref, not the working tree):

```
!`git show origin/HEAD:.ai/security-review-false-positives.md`
```

SECURITY MODEL (from the base ref, not the working tree):

```
!`git show origin/HEAD:docs/explanation/security-model.md`
```

ALLOWANCES (from the base ref, not the working tree):

```
!`git show origin/HEAD:docs/reference/security-allowances.md`
```
