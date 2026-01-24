# Catchup

Summarize the recent changes on the current branch to bring me up to speed.

## Instructions

1. Run `git rev-parse --abbrev-ref HEAD` to get the current branch name
2. Run `git merge-base main HEAD` to find where this branch diverged from main
3. Run `git log --oneline <merge-base>..HEAD` to see all commits on this branch
4. Run `git diff --stat <merge-base>..HEAD` to see files changed with line counts
5. For each significantly changed file, briefly describe what the changes do

## Output Format

Provide a concise summary including:
- Branch name and how many commits ahead of main
- High-level description of what this branch is implementing/fixing
- List of key files changed and what was modified in each
- Any important context I should know before continuing work

Keep the summary focused and actionable - I want to quickly understand the state of this branch so I can continue working on it.
