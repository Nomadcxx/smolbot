# Skills Library — Plan A: Built-in Skill Files

**Date:** 2026-03-31
**Scope:** Content creation only — no Go code changes
**Companion plan:** Plan B (not yet written) covers skill discovery/UX improvements

---

## 1. Overview and Goals

Smolbot ships with 8 built-in skills today: `clawhub`, `cron`, `github`, `memory`, `skill-creator`, `summarize`, `tmux`, `weather`. The library has gaps in three areas:

1. **Core productivity reasoning** — no structured debugging, code review, research, or git workflow skills
2. **Channel-specific operating procedures** — agents working over WhatsApp, Telegram, Discord, or Signal have no built-in guidance on formatting rules, rate limits, failure recovery, or session management
3. **Delegation/parallelism** — agents have `task` and `wait` tools but no skill teaching them when and how to use them

This plan adds 11 new skills to fill all three gaps. Every file is pure markdown placed under `skills/<name>/SKILL.md`. The embedded filesystem loads them automatically at build time via `loadBuiltinSkills()` in `pkg/skill/registry.go` — no Go changes are needed.

**Success criteria:**
- `go build ./...` succeeds after adding the files
- `skills.list` gateway response includes all 11 new skill names
- TUI F1 > Skills panel shows all 11 new entries with their descriptions

---

## 2. Implementation Approach

### Single-pass content creation

Each skill file is written once to its final location. There are no migrations, no database changes, and no schema updates. The only operation is: create the directory, write `SKILL.md`.

### Quality bar

Skills are read by the agent (Claude), not by end users. Write them as standing instructions to yourself:
- Concrete and actionable — not vague guidance
- Cover failure modes explicitly, not just happy paths
- Channel skills must be comprehensive enough for autonomous recovery without asking the user
- Core productivity skills should encode the actual reasoning process, not just category labels
- Keep each skill under ~600 words; the agent reads all relevant skills on every turn

### Frontmatter fields used by the registry

```yaml
---
name: <slug>           # must match the directory name
description: <string>  # used by the agent to decide relevance; one line
always: false          # true = always injected into system prompt
---
```

All 11 new skills use `always: false` (injected on demand). None require the `always: true` treatment that would make them permanent system-prompt fixtures.

---

## 3. Skills to Implement

### Group A — Core Productivity (5 skills)

---

#### 3.1 `systematic-debugging`

**File:** `skills/systematic-debugging/SKILL.md`

**Description field:**
> Structured root cause analysis for bugs and unexpected behaviour. Use when encountering errors, test failures, or surprising results.

**Key content areas:**

1. **The 6-step process** — Reproduce → Isolate → Hypothesise → Test hypothesis → Fix → Verify fix. Number each step explicitly when working through a bug so it is easy to track and revise.

2. **Reproducing the failure** — Get the exact error message, exact inputs, and exact environment. Never reproduce from memory. Copy the actual stack trace. Check whether the failure is deterministic or intermittent.

3. **Isolating the failure** — Binary-search through the call stack. Disable half the code; if the bug disappears, it was in the half you removed. Narrow the reproducer to the smallest possible input.

4. **Forming and testing hypotheses** — State the hypothesis explicitly before testing it ("I think X is nil because Y"). Run exactly one experiment per hypothesis. Record the result even if it disproves you — disproof is progress.

5. **Reading error messages carefully** — Go errors are wrapped; read from the innermost cause outward. JavaScript stack traces list callers top-to-bottom. Python tracebacks list callers bottom-to-top. Never skim the error message.

6. **Checking assumptions** — List assumptions about the system before searching for a bug. Common wrong assumptions: "this function is always called", "this value is always set", "this goroutine is the only writer", "the test environment matches production".

7. **Checking recent changes** — When a bug is new, check `git log --oneline -20` and `git diff HEAD~1` before doing anything else. Most regressions live in the last commit.

8. **Not jumping to solutions** — Do not write code until the root cause is confirmed. Premature fixes mask the real problem and create two bugs.

9. **Go-specific patterns** — nil pointer dereference (check every pointer before use), goroutine leak (context not cancelled, channel never closed), race condition (use `-race`), silent error discard (`_ = err` is a red flag).

10. **JS/TS-specific patterns** — `undefined is not a function` (method called on wrong type), unhandled promise rejection, mutation of shared state in async code, stale closure in `useEffect`.

11. **Python-specific patterns** — mutable default argument (`def f(x=[])`), import side-effects, off-by-one in slice notation, `==` vs `is` for None.

12. **Verifying the fix** — Re-run the exact reproducer. Also run the full test suite. Check that the fix doesn't break adjacent behaviour.

---

#### 3.2 `code-review`

**File:** `skills/code-review/SKILL.md`

**Description field:**
> Review changed code for correctness, logic bugs, edge cases, security issues, and clarity. Use when asked to review a diff, PR, or specific file.

**Key content areas:**

1. **Review checklist by category** — Structure the review pass in this order: correctness first, then logic bugs and edge cases, then security, then naming and clarity, then style. Never lead with style.

2. **Correctness** — Does the code do what the PR description says? Are there off-by-one errors? Are error return values checked? Are resources (files, connections, goroutines) cleaned up?

3. **Logic bugs and edge cases** — Empty input, nil/null, zero values, negative numbers, very large inputs, concurrent access, timeout expiry, unexpected ordering. Ask: what happens if this function is called twice? What happens if the network drops here?

4. **Security issues** — Injection (SQL, shell, HTML), secrets hardcoded or logged, authentication bypass, missing authorisation check, unbounded resource consumption (no limit on loop or allocation), path traversal.

5. **Naming and clarity** — Can you understand the intent from the name alone? Are variable names a single letter where a word would help? Are function names verbs? Are boolean names positive (avoid `notDisabled`)?

6. **Distinguishing substance from style** — Style: formatting, whitespace, import ordering. These should be flagged only if they violate a project convention. Substance: logic, correctness, security. Always flag substance; flag style sparingly and non-blockingly.

7. **When to flag vs suggest** — Flag (must fix): correctness bugs, security issues, crashes, data loss. Suggest (nice to fix): clarity improvements, alternative approaches. Praise (no action needed): clever solutions, good test coverage, well-chosen names.

8. **Matching intent** — Read the PR description first. The most important question is: does this change do what was intended? If yes, review quality. If no, flag the mismatch first.

9. **Go-specific review points** — Error wrapping (`fmt.Errorf("...: %w", err)`), deferred close on error path, unexported fields in exported structs, goroutine cleanup on context cancellation.

---

#### 3.3 `web-research`

**File:** `skills/web-research/SKILL.md`

**Description field:**
> Multi-step research workflow: form a question, search multiple angles, cross-check sources, synthesise. Use when asked to research a topic, find current information, or verify a claim.

**Key content areas:**

1. **Forming a clear question** — Before searching, write down the exact question being answered. Vague questions produce vague results. If the question has multiple parts, split them.

2. **When to use `web_search` vs `web_fetch`** — Use `web_search` to find candidate sources. Use `web_fetch` to read a specific URL in full. Never rely on search snippet text alone — fetch the page to confirm the detail.

3. **Searching multiple angles** — Search the question directly, then search for counterarguments, then search for the most authoritative source (official docs, RFCs, academic papers). Three searches minimum for anything non-trivial.

4. **Evaluating source quality** — Prefer: official documentation, primary sources, peer-reviewed work, well-known technical publications. Be sceptical of: Stack Overflow answers older than 2 years for fast-moving topics, anonymous blog posts, SEO-farm content.

5. **Handling contradictory sources** — Note the contradiction explicitly. Check the date of each source. Prefer newer for version-specific information, prefer primary for specification questions. If genuinely unresolved, report the contradiction to the user rather than picking one.

6. **Cross-checking facts** — Any specific claim (version number, API signature, statistic) must be confirmed by at least two independent sources before reporting it as fact.

7. **Structuring research output** — Lead with the direct answer. Follow with evidence. End with caveats or open questions. Cite sources with URLs. Do not bury the answer in qualifications.

8. **When to stop searching** — Stop when you have a confident answer confirmed by independent sources, or when you have exhausted reasonable search angles and must report uncertainty. Do not search indefinitely.

---

#### 3.4 `sequential-thinking`

**File:** `skills/sequential-thinking/SKILL.md`

**Description field:**
> Step-by-step reasoning with explicit revision capability. Use for complex multi-step problems, architecture decisions, debugging sequences, and any problem where earlier conclusions may need updating.

**Key content areas:**

1. **When to use this skill** — Complex problems where the answer is not immediately obvious, architecture decisions with multiple trade-offs, debugging sequences with multiple hypotheses, problems where a wrong early assumption cascades.

2. **The core discipline** — Number every reasoning step. State what you know before each step. State what you conclude after each step. A step that revises an earlier conclusion should say so explicitly: "Revising step 3: I previously assumed X, but Y shows X is false."

3. **Breaking down problems** — Before step 1, list what the problem requires. Break it into sub-problems. Identify which sub-problems depend on each other. Work the independent ones first.

4. **Revising earlier conclusions** — It is correct and expected to revise. Never hide a revision. Say "Step 7 (revising step 3)" and explain the new evidence. Revision is a sign of good reasoning, not a mistake.

5. **Architecture decisions** — List the options. For each option, list pros, cons, and constraints. Eliminate options that violate hard constraints first. Choose by weighing remaining pros/cons, not by gut feel. Record the decision rationale.

6. **Debugging sequences** — Apply this skill alongside `systematic-debugging`. Use numbered steps to track the investigation. Revise hypotheses explicitly when experiments disprove them.

7. **When to stop** — Stop when you have a conclusion supported by all the available evidence with no outstanding contradictions. If contradictions remain, surface them rather than suppressing them.

---

#### 3.5 `git-operations`

**File:** `skills/git-operations/SKILL.md`

**Description field:**
> Common git workflows: committing, branching, rebasing, conflict resolution, undoing mistakes, PR hygiene. Use when asked to perform git operations or when about to commit/push code.

**Key content areas:**

1. **Commit message format** — Use conventional commits: `type(scope): subject`. Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, `ci`. Subject: imperative mood, no period, max 72 chars. Body: wrap at 72 chars, explain why not what.

2. **When to amend vs new commit** — Amend only if the commit has not been pushed to a shared branch. After push, always create a new commit. Amending pushed commits forces others to rebase.

3. **Branching strategy** — Feature branches: `feat/<slug>`. Bug fix branches: `fix/<slug>`. Keep branches short-lived. Merge or rebase into main frequently. Delete branches after merge.

4. **Rebase vs merge** — Rebase for keeping a clean linear history on feature branches before PR. Merge for integrating into main (preserves topology). Never rebase a shared branch that others have checked out.

5. **Resolving conflicts** — Read both sides before choosing. `ours` means the current branch; `theirs` means the incoming branch. If unsure which is correct, ask — do not guess. After resolving, `git add` the file and continue the rebase or merge.

6. **Undoing mistakes** — `git restore <file>`: discard unstaged changes. `git reset HEAD <file>`: unstage without discarding. `git reset --soft HEAD~1`: undo last commit, keep changes staged. `git reset --hard` destroys work — confirm before using. `git revert <hash>`: safe undo for pushed commits.

7. **Stash usage** — `git stash` when you need to switch context mid-work. `git stash pop` to restore. Name stashes: `git stash save "wip: description"`. List with `git stash list`.

8. **Cherry-pick** — Use to port a specific commit to another branch: `git cherry-pick <hash>`. Use sparingly; prefer rebase for sequential commits. If the cherry-pick produces conflicts, resolve them the same way as merge conflicts.

9. **PR hygiene** — Squash noise commits before opening a PR. Ensure the PR description explains the why, not just the what. Link to the issue. Keep PRs small — one concern per PR.

10. **Checking before committing** — Always run `git diff --staged` before `git commit` to confirm exactly what is going into the commit.

---

### Group B — Channel Skills (5 skills)

---

#### 3.6 `whatsapp`

**File:** `skills/whatsapp/SKILL.md`

**Description field:**
> Operating procedures for the WhatsApp channel: formatting, message limits, media, rate limiting, session management, and failure recovery.

**Key content areas:**

1. **Formatting rules** — WhatsApp uses its own markup, not standard Markdown. Bold: `*text*`. Italic: `_text_`. Strikethrough: `~text~`. Monospace/code: `` `text` `` (inline) or ` ```text``` ` (block). Standard Markdown (`**bold**`, `__italic__`) is rendered as literal characters. Never use standard Markdown in WhatsApp messages.

2. **Message length limit** — 4096 characters per message. For responses exceeding this, split at a natural boundary (end of sentence or paragraph). Prefix continuation messages with "(continued)" so the user knows more is coming. Never truncate mid-sentence.

3. **Splitting strategy** — Plan the split before sending: count characters, find the last paragraph break under 4000 chars (leaving headroom), send that, then send the remainder. For code blocks that span a split, close the block in the first message and open a new one in the second.

4. **Media types and limits** — Supported: images (JPEG, PNG), PDF documents, audio (MP3, OGG, OPUS), video (MP4), stickers (WebP). File size limits: images 5 MB, documents 100 MB, audio 16 MB, video 16 MB. If a file exceeds the limit, split it or provide a link instead.

5. **Rate limiting** — WhatsApp enforces approximately 20 messages per minute per session. If sending multiple messages in quick succession (e.g., a long response split into 4 parts), insert a brief pause between messages. If rate-limited, back off for 60 seconds before retrying.

6. **Session management** — WhatsApp Web sessions expire after ~14 days of inactivity, or immediately if the phone's WhatsApp is unlinked. Signs of session expiry: message delivery fails with an auth error, `web_whatsapp_status` returns `unauthenticated`, or the user reports not receiving messages. Recovery: instruct the user to run `smolbot qr` to regenerate the QR code and re-link by scanning with WhatsApp on their phone.

7. **Detecting re-link needed** — If the send tool returns an error containing "session", "unauthenticated", "disconnected", or "QR", the session has expired. Tell the user: "WhatsApp session expired. Please run `smolbot qr` on the server and scan the QR code with WhatsApp on your phone."

8. **Group vs DM behaviour** — In groups, the agent receives all messages, not just those directed at it. Filter by mention or a configured command prefix. In DMs, all messages are directed at the agent. Group messages may come from multiple senders; track sender identity.

9. **Read receipts and typing indicators** — WhatsApp supports typing indicators (two grey ticks = delivered, two blue ticks = read). The agent can send a typing indicator before long operations to signal it is working.

10. **Fallback strategies** — If a message fails to deliver: retry once after 10 seconds. If the retry fails: check session status. If the session is valid: report the delivery failure to the user. If the session is invalid: prompt re-link.

11. **Best practices** — Keep responses concise; WhatsApp is a mobile-first platform. Avoid walls of text. Use bullet points for lists. Use code blocks only for genuine code or commands. Sparingly use formatting — over-formatted messages look spammy.

---

#### 3.7 `telegram`

**File:** `skills/telegram/SKILL.md`

**Description field:**
> Operating procedures for the Telegram channel: formatting modes, message limits, editing, commands, keyboards, rate limits, and failure recovery.

**Key content areas:**

1. **Formatting modes** — Telegram supports three parse modes: `MarkdownV2`, `HTML`, and plain text. **Prefer HTML** for formatted output — it is less error-prone than MarkdownV2. HTML tags: `<b>bold</b>`, `<i>italic</i>`, `<code>code</code>`, `<pre>pre-formatted block</pre>`, `<a href="url">link</a>`. MarkdownV2 requires escaping these characters with `\`: `_ * [ ] ( ) ~ \` > # + - = | { } . !` — any unescaped special char causes a parse error. Use plain text if you are unsure about escaping.

2. **Message length limit** — 4096 characters per message. Same split strategy as WhatsApp: find the last paragraph break under 4000 chars. For code blocks that span a split, close and reopen the `<pre>` tag across messages.

3. **Editing and deleting messages** — Telegram supports `editMessageText` (update sent message) and `deleteMessage`. Use edit for: correcting a mistake in the last message, updating a status message as a task progresses, appending to an in-progress output. Edit is preferable to sending a follow-up correction.

4. **Bot commands** — Commands start with `/`. Standard commands: `/start` (initialise the bot), `/help` (show available commands). Custom commands are configured in BotFather. When a user sends `/start`, respond with a brief introduction and list of available commands.

5. **Groups vs channels vs DMs** — DMs: all messages are directed at the bot. Groups: bot receives messages only when mentioned (`@botname`) unless it has `can_read_all_group_messages` privilege. Channels: bot can post but not receive messages from members. Adjust behaviour per context.

6. **Inline keyboards and reply keyboards** — Inline keyboards attach buttons to a specific message. Reply keyboards replace the user's keyboard with buttons. Use inline keyboards for: confirming a destructive action, multi-choice selection. Use reply keyboards for: persistent navigation options. Remove reply keyboards when they are no longer needed.

7. **Rate limits** — Global: 30 messages per second across all chats. Per-chat: 1 message per second. Bulk notifications to many chats: max 30 chats per second. If sending a split response, space messages 1 second apart to stay within per-chat limits.

8. **Token management** — A Telegram bot token is a string of the form `123456789:ABCdef...`. A `401 Unauthorized` error means the token is invalid or revoked. A `403 Forbidden` error means the bot was blocked by the user or removed from the group. Detect these and surface them as configuration errors rather than retrying.

9. **Failure recovery** — `400 Bad Request: can't parse entities`: the message has a formatting error. Retry with parse mode set to plain text. `429 Too Many Requests`: back off for the duration specified in the `retry_after` field of the error response. `403 Forbidden` in a group: bot was removed; inform the user.

10. **File sending** — Send files with `sendDocument`, `sendPhoto`, `sendAudio`. Files under 50 MB can be sent by upload; files 50–2000 MB require a URL. Caption limit: 1024 characters.

11. **Common failure modes** — Bot not added to group: the `chat not found` error means the bot has no access to that chat. Bot lacks send permission: `not enough rights to send messages` — the group admin must grant the bot send permissions.

---

#### 3.8 `discord`

**File:** `skills/discord/SKILL.md`

**Description field:**
> Operating procedures for the Discord channel: message limits, embeds, formatting, threads, permissions, rate limits, and failure recovery.

**Key content areas:**

1. **Message length limit** — 2000 characters per regular message. Embed description: up to 4096 characters. For long responses, prefer an embed over chunked messages. If content exceeds a single embed description, use multiple fields (up to 25 per embed, 1024 chars per field).

2. **Discord markdown** — Discord uses its own markdown. Bold: `**text**`. Italic: `*text*` or `_text_`. Underline: `__text__`. Strikethrough: `~~text~~`. Code (inline): `` `code` ``. Code block: ` ```language\ncode\n``` `. Spoiler: `||text||`. Standard Markdown features not supported: tables, headings, lists with `-` work but `*` bullets render as italic.

3. **Embeds** — Use embeds for: structured information, long output, responses with a title and body, results from a task. Embed anatomy: title (256 chars), description (4096 chars), fields (name 256 chars, value 1024 chars, up to 25 fields), footer (2048 chars), colour (hex). Total embed size must not exceed 6000 characters.

4. **Slash commands vs prefix commands** — New Discord bots use slash commands (`/command`). Prefix commands (`!command`) are legacy. Respond to the interaction that triggered the slash command within 3 seconds, or acknowledge it with a deferred response first; failing to respond within 3 seconds causes "This interaction failed" on the user's end.

5. **Threads** — Create a thread when: the conversation is going long, the content is a detailed investigation, or the response would otherwise clutter a busy channel. Use `startThread` on the original message. Subsequent replies go into the thread. Threads archive automatically after inactivity.

6. **Server vs DM context** — In a server channel, the bot can only see messages in channels it has `VIEW_CHANNEL` permission for. In DMs, all messages are visible. Guild (server) messages include `guild_id`; DM messages do not.

7. **Permissions** — Required permissions per channel: `VIEW_CHANNEL`, `SEND_MESSAGES`, `EMBED_LINKS` (for embeds), `ATTACH_FILES` (for file uploads), `CREATE_PUBLIC_THREADS` (for threads). Diagnose `Missing Permissions` errors by checking which permission is missing via the Discord developer portal or `GET /channels/{channel.id}`.

8. **Rate limits** — Per channel: 5 messages per 5 seconds. Globally: 50 requests per second. Discord returns a `429 Too Many Requests` with a `retry_after` field (in seconds) — always honour it. Do not retry before `retry_after` expires.

9. **Webhooks vs bot** — Webhooks are for one-way posting (notifications, CI results). A bot account supports two-way interaction. Do not conflate them: webhook tokens are different from bot tokens.

10. **Chunked message fallback** — If content exceeds the embed limit: split into multiple embeds, each sent as a separate message. Number them: "Result (1/3)". If even a single field exceeds 1024 chars, split that field across multiple fields.

11. **Common failure modes** — `50013 Missing Permissions`: bot lacks a required permission in that channel. `10003 Unknown Channel`: the channel ID is wrong or the bot has no access. `50007 Cannot send messages to this user`: the user has DMs disabled. `401 Unauthorized`: invalid bot token — requires reconfiguration.

---

#### 3.9 `signal`

**File:** `skills/signal/SKILL.md`

**Description field:**
> Operating procedures for the Signal channel: linked device model, plain text only, group behaviour, re-linking, disappearing messages, and failure recovery.

**Key content areas:**

1. **Linked device model** — smolbot connects to Signal as a linked device via `signal-cli`, not as a bot API. This means it runs as an additional device linked to a real Signal account (a phone number). There is no bot token; there is a device registration. This has implications: the device can be revoked from the primary phone at any time, and re-linking requires physical access to a QR code flow.

2. **No markdown formatting** — Signal renders all text as plain text. Do not use `*bold*`, `_italic_`, code fences, or any markdown. Structure responses with plain punctuation and whitespace: capitalised headings, numbered lists, blank lines between sections.

3. **Message length** — Signal has no hard per-message character limit in practical use, but keep messages concise. Long walls of text are difficult to read on mobile. For long content, break it into logical paragraphs with blank lines between them.

4. **Group vs DM behaviour** — In groups, the agent receives all messages from all members. Unless configured to respond to all group messages, respond only when directly addressed (name mention or configured prefix). In DMs, all messages are directed at the agent.

5. **Disappearing messages** — If the conversation has disappearing messages enabled, both incoming and outgoing messages will be deleted after the configured timer. The agent cannot disable this setting. Be aware that: messages in a disappearing conversation may be gone before the user can act on them; do not rely on the user having access to a previous response. For important information, ask the user to copy it before it disappears.

6. **Session expiry and re-linking** — The linked device registration expires if: the primary phone revokes the device, `signal-cli` loses state, or the account is re-registered. Signs of expiry: `signal-cli` returns an authentication error, messages are not delivered, the user reports the bot is unresponsive. Recovery: instruct the user to run the re-link flow: `smolbot qr` generates a new QR code; the user scans it in Signal on their phone under Settings > Linked Devices > Link New Device.

7. **Detecting session expiry** — If the send operation returns an error containing "NotLinked", "AuthorizationFailedError", "Invalid registration", or "device not found", the session has expired. Report this to the user immediately: "Signal session expired. Please run `smolbot qr` and scan the QR code in Signal > Settings > Linked Devices."

8. **Privacy implications** — Signal provides end-to-end encryption. Messages are not stored on Signal's servers after delivery. The agent should not log message content to disk or external services, as this would undermine the privacy expectation of Signal users.

9. **Attachments** — Signal supports images (JPEG, PNG), documents (PDF, etc.), and audio. Send via `signal-cli send --attachment <path>`. There is no documented hard size limit in signal-cli, but keep attachments under 100 MB to avoid delivery issues.

10. **Common failure modes** — `signal-cli not running`: check systemd service status (`systemctl status smolbot-signal`); restart if needed. `Device not linked`: run re-link flow. `Number not registered on Signal`: the target number does not have a Signal account; this is not a recoverable error — inform the user. `Rate limited`: Signal rate-limits message sending; back off for 60 seconds.

---

### Group C — Cross-Channel Skill (1 skill)

---

#### 3.10 `channel-triage`

**File:** `skills/channel-triage/SKILL.md`

**Description field:**
> Routing and triage across multiple messaging channels: when to use which channel, how to handle simultaneous inbound, consistent identity, escalation paths, and long-form content routing.

**Key content areas:**

1. **Channel characteristics matrix:**
   - WhatsApp: casual/mobile, high engagement, rich media support, 4096 char limit, session requires QR re-link
   - Telegram: tech/automation-friendly, bot commands, inline keyboards, editable messages, 4096 char limit, persistent bot token
   - Discord: community/dev, slash commands, embeds, threads for long content, 2000 char limit (4096 in embeds), server permissions model
   - Signal: privacy-focused, E2E encrypted, plain text only, no markdown, linked device model, disappearing messages possible

2. **Routing heuristics by message type:**
   - Sensitive content (credentials, personal data, private decisions): Signal first, WhatsApp second
   - Long technical output (code, logs, reports): Telegram (edit support, code blocks) or Discord (embeds, threads)
   - Quick casual requests: WhatsApp or Telegram DM
   - Community/group announcements: Discord channel
   - Automation triggers and cron notifications: Telegram (reliable, persistent token)

3. **Routing by user preference** — If the user has expressed a channel preference, honour it over heuristics. When unknown, ask once and store the preference in memory.

4. **Handling simultaneous inbound** — If requests arrive on multiple channels at the same time: process each independently. Do not merge sessions. Each channel maintains its own conversation context. If the same user sends the same request on two channels, handle both and note the duplication if appropriate.

5. **Consistent identity** — The agent presents the same persona across channels. Formatting adapts to the channel's capabilities but tone and capabilities do not change. Do not promise a capability on one channel that is unavailable on another.

6. **When to ask the user for preferred channel** — Ask when: the user initiates on a channel that is a poor fit for the content type (e.g., a long code review requested on WhatsApp), or when a task will generate a lot of follow-up and a more capable channel is available.

7. **Escalation paths** — If the primary channel fails (session expired, rate limited, delivery error):
   - WhatsApp → Telegram (if configured) → Email or manual fallback
   - Telegram → WhatsApp (if configured)
   - Discord → Telegram (if configured)
   - Signal → no fallback (by design — privacy-conscious users may not want fallback)

8. **Long-form content routing** — For content over 2000 characters: prefer Discord (embeds/threads) or Telegram (editable messages, HTML formatting). For Signal or WhatsApp, break into multiple messages with explicit continuation markers.

---

### Group D — Delegation Skill (1 skill)

---

#### 3.11 `task-delegation`

**File:** `skills/task-delegation/SKILL.md`

**Description field:**
> When and how to use the `task` and `wait` tools to delegate sub-tasks to parallel child agents. Use when a problem has independent sub-tasks that can be worked concurrently.

**Key content areas:**

1. **The `task` tool** — Delegates a structured sub-task to a background child agent. Required parameters:
   - `description` (string): short label shown in the UI, e.g. "Fetch upstream API docs"
   - `prompt` (string): the full instructions the child agent will receive
   - `agent_type` (string): the type of agent to spawn (use `"default"` unless a specific type is needed)
   - Optional: `model` (string), `reasoning_effort` (string: `"low"`, `"medium"`, `"high"`)
   - Returns: a delegated task ID (via `agentID` in metadata) used with `wait`

2. **The `wait` tool** — Waits for one or more delegated child agents to finish. Parameter:
   - `agent_ids` (array of strings, optional): specific agent IDs to wait for; if omitted, waits for all outstanding children
   - Returns: `count` (number of agents completed) and `results` (array of agent result summaries)

3. **Identifying parallelisable sub-tasks** — Sub-tasks are parallelisable if: they do not depend on each other's output, they can be started with information available right now, and their results can be merged afterwards. Examples: "fetch these 5 URLs", "run these 3 independent searches", "analyse these 4 log files".

4. **Writing clear task prompts** — The child agent receives only what is in `prompt` — it has no access to the parent conversation. Write the prompt as a complete self-contained instruction: include all context, specify the expected output format, and state what the child should return. A bad prompt: "check the logs". A good prompt: "Read the file at /var/log/app.log. Find all ERROR-level lines from the past 24 hours. Return them as a JSON array with fields: timestamp, message, stack_trace (if present)."

5. **Worked example — good task decomposition:**
   - Problem: "Summarise what changed in these 4 GitHub repos since last week"
   - Good decomposition: spawn 4 tasks in parallel, one per repo, each fetching and summarising independently, then `wait` for all 4 and merge
   - Bad decomposition: one task that fetches all 4 repos sequentially (no parallelism benefit)

6. **Worked example — bad task decomposition:**
   - Problem: "Debug why the API is slow and then write a fix"
   - Bad: delegate "debug" to task A and "write fix" to task B in parallel — task B cannot write the fix before task A identifies the cause
   - Correct: run the debugging sequentially, then delegate the fix writing only after the root cause is known

7. **Choosing agent_type** — Use `"default"` for general-purpose tasks. Other types (if configured in the deployment) may be specialised for specific domains. Check available types via the operator.

8. **When to wait vs fire-and-forget** — Always `wait` if you need the results to continue your work. Fire-and-forget (no explicit `wait`) is appropriate only for tasks whose results you will not use in this turn, e.g. a background cache warm-up. Note: `task` disables the child's `message`, `spawn`, and `task` tools — child agents cannot spawn further children or message the user.

9. **Interpreting wait results** — Check `count` matches the number of tasks spawned. Iterate over `results`. Each result has a summary of the child's output. If a child failed, its result will contain the error. Handle partial failure: if 3 of 4 tasks succeeded, report the 3 results and surface the failure for the 4th.

10. **Failure handling** — If `task` returns an error ("spawner unavailable", "session key required"): these are infrastructure errors; report them to the user rather than retrying. If a child agent times out or errors: `wait` will still return with the partial results; surface the failure in your response.

---

## 4. Verification Steps

After creating all 11 skill files, verify in this order:

### 4.1 Build check

```
cd /path/to/smolbot && go build ./...
```

If this fails with a path error on `skills/`, it means the embedded filesystem glob in `embed.go` does not cover the new directories. Check that the embed directive covers `skills/**` recursively.

### 4.2 Gateway response

Send a `skills.list` request to the running daemon:
```
smolbot skills
```
or via raw WebSocket:
```json
{"id": 1, "method": "skills.list", "params": {}}
```

Expected: response contains all 11 new skill names alongside the existing 8.

### 4.3 TUI verification

Open the TUI, press F1, navigate to Skills. All 11 new skills should appear with their one-line descriptions. Selecting each skill should show the full skill content.

### 4.4 Content spot-check

For each skill, confirm:
- Frontmatter parses without error (name, description fields present)
- The `name` field matches the directory name
- No standard Markdown tables are used in channel skills (they won't render on those channels)
- Word count is reasonable (200–600 words per skill)

---

## 5. Commit Strategy

Three logical commits, one per group:

### Commit 1 — Core productivity skills

Files:
- `skills/systematic-debugging/SKILL.md`
- `skills/code-review/SKILL.md`
- `skills/web-research/SKILL.md`
- `skills/sequential-thinking/SKILL.md`
- `skills/git-operations/SKILL.md`

Message: `feat(skills): add core productivity skills (debugging, code-review, web-research, sequential-thinking, git-operations)`

### Commit 2 — Channel skills

Files:
- `skills/whatsapp/SKILL.md`
- `skills/telegram/SKILL.md`
- `skills/discord/SKILL.md`
- `skills/signal/SKILL.md`
- `skills/channel-triage/SKILL.md`

Message: `feat(skills): add channel operating procedure skills (whatsapp, telegram, discord, signal, channel-triage)`

### Commit 3 — Delegation skill

Files:
- `skills/task-delegation/SKILL.md`

Message: `feat(skills): add task-delegation skill with task/wait tool reference and worked examples`

---

## 6. Notes and Constraints

- All 11 skills use `always: false`. None are injected unconditionally.
- The `sequential-thinking` skill here is a smolbot built-in operating instruction. It is distinct from any external MCP tool or plugin with the same name.
- The `code-review` skill here is for reviewing code diffs. It is distinct from skills that process incoming code review feedback on the agent's own output.
- Channel skills must be updated if smolbot adds a new channel adapter (e.g., Matrix, Slack). A corresponding skill should be added in the same commit as the adapter.
- The `task-delegation` skill references tool parameter names that are defined in `pkg/tool/task.go` and `pkg/tool/wait.go`. If those parameter names change, update this skill in the same PR.
- No skill in this library requires a new MCP tool, new config key, or new Go package. Creation of the markdown files is the entire implementation.
