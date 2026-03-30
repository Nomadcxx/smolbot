---
name: github
description: Interact with GitHub repositories for code search, file operations, and repository management. Use when browsing GitHub code, searching repositories, or managing GitHub resources.
---

Use this skill when you need to interact with GitHub repositories.

## When to Use

- Search for code across GitHub repositories
- Read file contents from a repository
- Get repository metadata and information
- Browse repository structure
- Search for specific functions, classes, or patterns

## Common Patterns

### Search Code
Search for specific code patterns across repositories using GitHub search API.

### Read File
Retrieve file contents from a specific path in a repository at a given ref (branch, tag, or commit).

### Get Repository Info
Retrieve metadata about a repository including description, language, stars, and recent activity.

## GitHub URL Format

- Repository: `owner/repo`
- File: `owner/repo/path/to/file`
- Search: `owner/repo` with search query

## Best Practices

- Always specify the repository owner and name clearly
- Include branch/ref when you need a specific version
- Use search to find existing implementations before adding new code
