# Agent Instructions

## Code Conventions & Idioms

Rigorous adherence to these patterns is required:

* **Reuse:**
    * Avoid code duplication.
    * Use existing functions if possible.
    * Create a function if code is used more than two times.
* **Testing:**
    * **Test-Driven Development (mandatory):** Never change production code without first writing a failing test that captures the new behaviour. Write the test, see it fail, then make it pass. This applies to bug fixes (regression test first), new features, and contract changes (e.g. allowing an anomaly return value).

## Release Notes

When generating a `CHANGELOG.md` entry for a new version:

1. **Fetch issues** from the GitHub milestone matching the version name: `gh issue list --milestone <version> --state all --limit 200 --json number,title,labels,state,url`
2. **Fetch the milestone number** for the "full changelog" link: `gh api repos/samply/blazectl/milestones --jq '.[] | select(.title == "<version>") | .number'`
3. **Categorize** issues by their labels into sections — in this order if present: `Notes`, `Survey`, `Security`, `Enhancements`, `Performance`, `Bugfixes`, `Documentation`, `Maintenance`. Omit issues labeled `ci` and dependency updates.
4. **Use issue titles verbatim** (1:1) as the changelog entry text. Do **not** rewrite, title-case, shorten, or normalize them to the imperative mood — even when an issue title does not follow the usual title conventions. Fixing the issue title is out of scope at release time.
5. **Insert** the new entry at the top of `CHANGELOG.md`, directly below the `# Changelog` heading, following the format of existing entries.
6. **End** the entry with: `The full changelog can be found [here](https://github.com/samply/blazectl/milestone/<number>?closed=1).`
    
## Verification & Workflow

When starting to work on an issue, you can use the GitHub CLI to fetch the issue details: `gh issue view <issue-number>`

When **creating** an issue, classify it via GitHub's native issue **type** (e.g. `Bug`, `Feature`), not via a `bug`/`feature` label. The `gh issue create` flag for this is `--type` (e.g. `gh issue create --type Bug ...`). Only labels that aren't covered by a type (e.g. `module:db`) should be passed via `--label`. Bugs in test code that can be fixed purely by changing **only** test code (e.g. a flaky test) aren't bugs, because the production code is unaffected. Such issues get the type `Task`, not `Bug`.

**Every change requires a GitHub issue.** If no issue exists for the work, create one first (see above) before writing any code. All work must be tracked by an issue.

After verification, when working on an issue:

1. Commit the changes: `git add .` and `git commit`
   * The commit title should be the issue title. Both use the imperative mood, are written in title case, and fit within about 50 characters (e.g. `Fix Error Combining Composite Token-Token Params`).
   * The commit body should just contain: `Closes: #<issue-number>`
   * Do **not** add a `Co-Authored-By` trailer (or any other AI/tool attribution). The changes are authored by the human, who appears as the commit's author and uses AI merely as a tool; the AI assistant may appear only as the commit's committer.
2. There should be exactly one commit per issue. Multiple changes have to be ammended to the first commit.
