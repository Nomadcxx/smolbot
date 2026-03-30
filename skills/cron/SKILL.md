---
name: cron
description: Schedule and manage periodic tasks with cron-style scheduling. Use when setting up recurring jobs, managing scheduled commands, or handling time-based automation.
---

Use this skill when you need to work with cron-style job scheduling.

## When to Use

- Set up a new recurring task or job
- List or view existing scheduled jobs
- Modify or remove a scheduled job
- Understand cron schedule expressions
- Debug scheduling issues

## Cron Expression Format

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
* * * * *
```

## Schedule Examples

| Expression | Meaning |
|------------|---------|
| `0 * * * *` | Every hour at minute 0 |
| `*/15 * * * *` | Every 15 minutes |
| `0 9 * * 1-5` | Weekdays at 9 AM |
| `0 0 * * *` | Daily at midnight |
| `30 4 * * 0` | Sunday at 4:30 AM |

## Job Fields

- **action**: What to execute (e.g., "http_get", "shell")
- **schedule**: Cron expression
- **timezone**: Timezone for scheduling (default: local)
- **name**: Human-readable job name
- **id**: Unique job identifier
- **enabled**: Whether the job is active
