# Agent Instructions

## Code Conventions & Idioms

Rigorous adherence to these patterns is required:

* **Reuse:**
    * Avoid code duplication.
    * Use existing functions if possible.
    * Create a function if code is used more than two times.
* **Testing:**
    * **Test-Driven Development (mandatory):** Never change production code without first writing a failing test that captures the new behaviour. Write the test, see it fail, then make it pass. This applies to bug fixes (regression test first), new features, and contract changes (e.g. allowing an anomaly return value).
    
## Verification & Workflow

When starting to work on an issue, you can use the GitHub CLI to fetch the issue details: `gh issue view <issue-number>`

When **creating** an issue, classify it via GitHub's native issue **type** (e.g. `Bug`, `Feature`), not via a `bug`/`feature` label. The `gh issue create` flag for this is `--type` (e.g. `gh issue create --type Bug ...`). Only labels that aren't covered by a type (e.g. `module:db`) should be passed via `--label`.

After verification, when working on an issue:

1. Create a feature branch using the GitHub CLI: `gh issue develop <issue-number> --checkout`
2. Commit the changes: `git add .` and `git commit`
    * The commit title should be the issue title.
    * The commit body should just contain: `Closes: #<issue-number>`
3. There should be exactly one commit per issue. Multiple changes have to be ammended to the first commit.
