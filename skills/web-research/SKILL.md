---
name: web-research
description: Multi-step research workflow: form a question, search multiple angles, cross-check sources, synthesise. Use when asked to research a topic, find current information, or verify a claim.
always: false
---

## Web Research

Research systematically. Stop when you have a confident answer confirmed by independent sources.

### Form a Clear Question

Before searching, write down the exact question being answered. Vague questions produce vague results. If the question has multiple parts, split them.

### web_search vs web_fetch

Use `web_search` to find candidate sources. Use `web_fetch` to read a specific URL in full. Never rely on search snippet text alone — fetch the page to confirm the detail.

### Search Multiple Angles

Search the question directly, then search for counterarguments, then search for the most authoritative source (official docs, RFCs, academic papers). Three searches minimum for anything non-trivial.

### Evaluate Source Quality

Prefer: official documentation, primary sources, peer-reviewed work, well-known technical publications.

Be sceptical of: Stack Overflow answers older than 2 years for fast-moving topics, anonymous blog posts, SEO-farm content.

### Handle Contradictory Sources

Note the contradiction explicitly. Check the date of each source. Prefer newer for version-specific information, prefer primary for specification questions. If genuinely unresolved, report the contradiction rather than picking one.

### Cross-Check Facts

Any specific claim (version number, API signature, statistic) must be confirmed by at least two independent sources before reporting it as fact.

### Structure Research Output

Lead with the direct answer. Follow with evidence. End with caveats or open questions. Cite sources with URLs. Do not bury the answer in qualifications.

### When to Stop

Stop when you have a confident answer confirmed by independent sources, or when you have exhausted reasonable search angles and must report uncertainty. Do not search indefinitely.
