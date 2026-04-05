# Mid-Session Triggers

## Principle

Mid-session lookup should be selective. Query memory only when there is evidence that prior context could change current action.

## Trigger Signals

Use memory lookup when:

- a bug or failure pattern repeats
- you suspect "we solved this before"
- a project-specific convention likely exists
- a user preference materially affects execution
- an RCA or architecture decision needs prior context
- a benchmark or prior result may change next steps

## Retrieval Pattern

Use a staged flow:

1. narrow query first
2. inspect only the most relevant matches
3. pull full detail only for shortlisted items

## Avoid Lookup When

- the task is routine and local
- there is no sign that prior context matters
- the query would be broad and noisy
- you are just filling silence with extra actions
