# Release Command

Create a release PR with summarized changes from merged pull requests.

## Steps

1. Switch to the main branch if not already
1. Run the ./scripts/upgrade.bash script to get the list of merged PRs caused by upgrading openapi-generation dep
1. **IMPORTANT: Filter out PRs which are internal changes. PRs with titles starting with "chore:" are almost definitely internal change. You can verify if it's an internal PR or public PR by reading any files changed under `changelogs/` in the PR. If no files were changed under this directory, this is an internal PR. Read the `changelogs/` directory for more context.
1. You should use `gh` to go and read the descriptions and `changelogs/` directory.
1. Create a user-facing summary for each language with relevant changes
1. Group changes by programming language (python, typescript, java, go, csharp, php, ruby, terraform)
1. Ignore "v2" suffixes when categorizing (pythonv2 → python, typescriptv2 → typescript)
1. **Create a PR title that includes**:
   - Which languages have been updated
   - Whether changes are features or fixes
   - Brief description of what was changed
   - Format: `feat(lang1): brief description of feature changes; fix(lang1) description of fixes; feat(lang2) ....`
1. Create a new branch off main with format: `release/vX.Y.Z` or `release/YYYY-MM-DD-HH:mm`
1. Commit the changes with the summarized message
1. Push the branch and create a PR with the full changelog



The PR description should be formatted as:
**IMPORTANT** exclude 🤖 Generated with [Claude Code](https://claude.com/claude-code) from PR descriptions
```markdown
## Core
- [If any changes](link to pr)

## Python
- [Brief user-facing summary of changes](link to pr)

## TypeScript
- [Brief user-facing summary of changes](link to pr)

## {lang3 }...
...
```

