# Creating Skills

## Quick Start

Create a new skill in your user skills directory:

```bash
mkdir -p ~/.smolbot/skills/my-skill
cat > ~/.smolbot/skills/my-skill/SKILL.md << 'EOF'
---
name: my-skill
description: Use when performing specific task X in domain Y
---

# My Skill

## When to Use

Use this skill when:
- Condition 1 requiring this skill
- Condition 2 requiring this skill

## How to Use

1. Step one
2. Step two
3. Step three
EOF
```

## Description Format

The `description` field is critical and must follow strict format:

### Required Format
```
description: Use when [specific trigger conditions]
```

### Good Examples

- ✅ `Use when checking weather forecasts or current conditions for a location`
- ✅ `Use when deploying applications to Cloudflare Workers with wrangler`
- ✅ `Use when processing multimedia files with FFmpeg for encoding, conversion, or filtering`

### Bad Examples

- ❌ `This skill provides weather information` (doesn't start with "Use when")
- ❌ `A skill for working with databases` (describes what, not when)
- ❌ `Use this skill to deploy code` (describes purpose, not trigger)

## Frontmatter Fields

```yaml
---
name: skill-name                  # Required: lowercase letters, numbers, hyphens
                                  # Max 64 characters
                                  
description: Use when...          # Required: trigger conditions starting with "Use when"
                                  # Max 500 characters
                                  
always: false                     # Optional: load into every context (default: false)

requires:                         # Optional: requirements check
  bins:                           #   Required CLI binaries
    - node
    - python3
  env:                            #   Required environment variables
    - API_KEY
    - AWS_REGION
---
```

## Directory Structure

### Simple Skill

```
my-skill/
└── SKILL.md
```

### Complex Skill with Resources

```
my-skill/
├── SKILL.md
├── references/
│   ├── api-docs.md
│   └── schema.json
├── scripts/
│   ├── helper.py
│   └── setup.sh
└── assets/
    └── template.yaml
```

## Skill Locations (Priority Order)

Skills are loaded from three locations. Later sources override earlier ones:

1. **Builtin** (`embed:`) - Built into nanobot binary
2. **User** (`~/.smolbot/skills/`) - Your personal skills
3. **Workspace** (`<workspace>/skills/`) - Project-specific skills

Example: If you have `docker` skill in all three locations, the workspace version wins.

## Loading Skills

### The 1% Rule

**If there is even a 1% chance a skill might apply to what you're doing, ABSOLUTELY load it.**

Better to load an unnecessary skill than miss a relevant one.

### How to Load

Skills are loaded by reading the SKILL.md file directly:

```
read_file(skills/{skill-name}/SKILL.md)
```

After loading, follow the instructions in the skill.

### Available Skills

Skills are listed in the `<system-skills>` section of the system prompt with metadata only:
- `name` - Skill identifier
- `status` - "available" or "unavailable"
- `reason` - Why unavailable (if applicable)
- `always` - Whether pre-loaded into every context

## Bundled Resources

Skills can include additional files in subdirectories:

### `references/` - Documentation

Load these when referenced by the skill:

```
my-skill/references/
├── api-spec.md      # API documentation
├── schema.sql       # Database schema
└── examples/        # Code examples
    └── sample.go
```

### `scripts/` - Executable Helpers

Run these when instructed by the skill:

```
my-skill/scripts/
├── deploy.sh        # Deployment script
├── validate.py      # Validation script
└── setup.js         # Setup script
```

### `assets/` - Templates and Assets

Read/copy these when needed:

```
my-skill/assets/
├── template.yaml    # Config template
├── boilerplate/     # Project boilerplate
└── icon.png         # Resource file
```

## Testing Your Skill

1. **Create the skill** in `~/.smolbot/skills/<skill-name>/`
2. **Restart nanobot** or run `nanobot chat` fresh session
3. **Check it's available**: Look for it in `<system-skills>` output
4. **Test loading**: Verify `read_file(skills/<skill-name>/SKILL.md)` works
5. **Test usage**: Try a query that should trigger the skill

## Example: Weather Skill

```markdown
---
name: weather
description: Use when checking weather forecasts or current conditions for a location
---

# Weather Skill

## When to Use

Use when the user asks about:
- Current weather conditions
- Weather forecasts
- Temperature, humidity, precipitation
- Weather alerts or warnings

## How to Use

1. Use `web_search` to find current weather information
2. Use `web_fetch` to get detailed forecast if needed
3. Provide clear, concise weather summary

## Limitations

- Does not provide historical weather data
- Does not set up weather alerts or notifications
```

## Example: Docker Skill

```markdown
---
name: docker
description: Use when working with Docker containers, images, or Compose files
---

# Docker Skill

## When to Use

Use when:
- Building Docker images
- Running containers
- Writing or editing docker-compose.yml
- Troubleshooting container issues

## When NOT to Use

- For Kubernetes (use kubernetes skill instead)
- For CI/CD pipelines (use devops skill instead)

## How to Use

1. Check if Dockerfile/docker-compose.yml exists with `list_dir`
2. Use bundled scripts for complex operations:
   - Run `scripts/cleanup.sh` to remove unused resources
   - Run `scripts/health-check.sh` to verify container health

## References

- `references/dockerfile-best-practices.md` - Best practices guide
- `references/compose-examples/` - Example compose files for common setups
```

## Best Practices

1. **Keep descriptions concise** - Under 500 characters, focus on triggers
2. **Use "Use when" format** - Always start with "Use when"
3. **Be specific about triggers** - What makes this skill activate?
4. **Include limitations** - When should this skill NOT be used?
5. **Bundle reusable resources** - Scripts, templates, references
6. **Follow 1% rule** - Better to load unnecessarily than miss relevant skill
7. **Test thoroughly** - Verify skill loads and works as expected

## Troubleshooting

### Skill not appearing in `<system-skills>`

- Check directory path: `~/.smolbot/skills/<name>/SKILL.md`
- Verify YAML frontmatter is valid (use `---` delimiters)
- Ensure required fields: `name` and `description`

### "Skill not found" when loading

- Check skill name matches directory name
- Verify file is named `SKILL.md` (case-sensitive)
- Check file permissions (readable by nanobot)

### Description not matching trigger

- Must start with "Use when"
- Describe trigger conditions, not functionality
- Keep under 500 characters
