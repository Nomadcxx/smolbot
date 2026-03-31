---
name: skill-creator
description: Creates new skills for smolbot. Use when the user asks you to build, create, add, or install a skill.
always: false
---

# Creating Skills

A skill is a markdown file that gives the agent standing instructions for a topic. Skills live at `~/.smolbot/skills/<name>/SKILL.md` (user skills) or in the embedded `skills/` directory (builtin skills).

## File Format

```yaml
---
name: <slug>
description: <one-line description — used to decide if the skill is relevant>
always: false   # true = always injected; false = injected only when relevant
---
```

The body is plain markdown. Write it as instructions to yourself: what to do, when to use specific tools, concrete examples.

## Writing Good Skill Content

- Lead with **when to trigger** this skill (user phrases, task types)
- Include **concrete tool invocations** — show the exact JSON arguments, not just descriptions
- Add **example inputs and outputs** so the agent knows what success looks like
- Keep it under ~400 words; the agent reads every relevant skill on each turn

## Steps to Create a Skill

1. Choose a slug: lowercase, hyphenated (e.g. `git-workflow`)
2. Create the directory: `~/.smolbot/skills/git-workflow/`
3. Write `SKILL.md` with the frontmatter above and a substantive body
4. Reload skills: use `smolbot reload` or restart the daemon

## Example Skill

```markdown
---
name: git-workflow
description: Enforces commit conventions and branch naming for this repo
always: false
---

Always use conventional commits: `feat:`, `fix:`, `docs:`, `chore:`.
Branch names: `feat/<slug>`, `fix/<slug>`.

Before committing, run:
\`\`\`
exec {"command": "git diff --staged --stat"}
\`\`\`
```
