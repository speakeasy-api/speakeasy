# Release Command

Create a release PR with summarized changes from merged pull requests.

## Steps

0. Switch to the main branch if not already
1. Run the ./scripts/upgrade.bash script to get the list of merged PRs caused by upgrading openapi-generation dep
3. **IMPORTANT: Filter out PRs with titles starting with "chore:" - these MUST be excluded from both the PR title and description**
7. You should use `gh` to go and read the descriptions of each and every PR to workout if this is a public facing change or internal change. This PR should only include public changes.
6. Create a user-facing summary for each language with relevant changes
4. Group changes by programming language (python, typescript, java, go, csharp, php, ruby, terraform)
5. Ignore "v2" suffixes when categorizing (pythonv2 → python, typescriptv2 → typescript)
8. **Create a PR title that includes**:
   - Which languages have been updated
   - Whether changes are features or fixes
   - Brief description of what was changed
   - Format: `feat(lang1): brief description of feature changes; fix(lang1) description of fixes; feat(lang2) ....`
9. Create a new branch off main with format: `release/vX.Y.Z` or `release/YYYY-MM-DD`
10. Commit the changes with the summarized message
11. Push the branch and create a PR with the full changelog



The PR description should be formatted as:

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

