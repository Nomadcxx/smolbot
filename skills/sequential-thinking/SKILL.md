---
name: sequential-thinking
description: Step-by-step reasoning with explicit revision capability. Use for complex multi-step problems, architecture decisions, debugging sequences, and any problem where earlier conclusions may need updating.
always: false
---

## Sequential Thinking

Think in numbered steps. Revise explicitly when earlier conclusions change.

### When to Use

Use for: complex problems where the answer is not immediately obvious, architecture decisions with multiple trade-offs, debugging sequences with multiple hypotheses, problems where a wrong early assumption cascades.

### Core Discipline

Number every reasoning step. State what you know before each step. State what you conclude after each step. A step that revises an earlier conclusion should say so explicitly: "Revising step 3: I previously assumed X, but Y shows X is false."

### Break Down the Problem

Before step 1, list what the problem requires. Break it into sub-problems. Identify which sub-problems depend on each other. Work the independent ones first.

### Revising Earlier Conclusions

It is correct and expected to revise. Never hide a revision. Say "Step 7 (revising step 3)" and explain the new evidence. Revision is a sign of good reasoning, not a mistake.

### Architecture Decisions

List the options. For each option, list pros, cons, and constraints. Eliminate options that violate hard constraints first. Choose by weighing remaining pros/cons, not by gut feel. Record the decision rationale.

### Debugging Sequences

Apply this skill alongside `systematic-debugging`. Use numbered steps to track the investigation. Revise hypotheses explicitly when experiments disprove them.

### When to Stop

Stop when you have a conclusion supported by all the available evidence with no outstanding contradictions. If contradictions remain, surface them rather than suppressing them.
