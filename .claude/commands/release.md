# Release Command

Create a release PR with summarized changes from merged pull requests.

## Steps

0. Switch to the main branch if not already
1. Run the upgrade.bash script to get the list of merged PRs and upgrade dependencies
2. Fetch all merged PRs from openapi-generation repo since the last release
3. **IMPORTANT: Filter out PRs with titles starting with "chore:" - these MUST be excluded from both the PR title and description**
4. Group changes by programming language (python, typescript, java, go, csharp, php, ruby, terraform)
5. Ignore "v2" suffixes when categorizing (pythonv2 → python, typescriptv2 → typescript)
6. Create a user-facing summary for each language with relevant changes
7. **Create a PR title that includes**:
   - Which languages have been updated
   - Whether changes are features or fixes
   - Brief description of what was changed
   - Format: `feat/fix(lang1,lang2): brief description of changes`
8. Create a new branch off main with format: `release/vX.Y.Z` or `release/YYYY-MM-DD`
9. Commit the changes with the summarized message
10. Push the branch and create a PR with the full changelog

## PR Title Format

The PR title should follow this format with semicolons separating each language-specific change:
- `fix(python): remove template hack; feat(csharp): add cancellation tokens; fix(core): resolve security crashes`
- `feat(typescript): add retry logic; fix(go): resolve memory leaks`
- `fix(core): fix overlays and security; feat(python,csharp): add new features`

**Format Rules:**
- Separate each language's changes with semicolons (`;`)
- Each section should have its own type prefix (`feat` or `fix`)
- Group languages together only if they have the same type and similar changes
- Use "core" for general/WASM/infrastructure changes
- Order: fixes first, then features
- Be specific about what was fixed or added for each language

## Output Format

The PR description should be formatted as:

```markdown
## Release Summary

### Python
- [Brief user-facing summary of changes]

### TypeScript
- [Brief user-facing summary of changes]

### [Other Languages]
- [Brief user-facing summary of changes]

## Detailed Changes

[Full list of PR titles and links, excluding chore PRs]
```

## Important Notes

- **CRITICAL: Exclude any PRs with titles starting with "chore:" from BOTH the PR title and description**
- Focus on user-facing changes and improvements
- Group by language, ignoring v2 suffixes
- The branch should be created off main
- The PR should target main branch
- PR title must clearly indicate which languages were updated and what kind of changes (feat/fix)
