---
name: release-gwq
description: "Guide the gwq release process step by step: create release branch, write release notes, open PR, tag, and sync GitHub Release. Use this skill whenever the user wants to release a new version, cut a release, create a tag, bump version, or says /release-gwq. Takes a version argument like /release-gwq v0.0.15."
---

# Release

Automate the gwq release process. Takes a version argument (e.g., `/release-gwq v0.0.15`).

## Prerequisites

- `gh` CLI installed and authenticated
- Write access to `d-kuro/gwq`

## Release Flow

The release is a two-phase process. Phase 1 prepares the release (branch, notes, PR). Phase 2 happens after the PR is merged (tag, publish, sync notes). Guide the user through each step, confirming before destructive or visible actions.

### Phase 1: Prepare Release

#### Step 1 — Determine version

Parse the version from the argument. If not provided, ask the user. The version must follow semver format: `vX.Y.Z` (with leading `v`). If the user gives a version without the `v` prefix, add it.

Find the previous tag to use for changelog comparison:

```bash
git tag --sort=-v:refname | head -1
```

If no previous tag exists, use the first commit as the base.

#### Step 2 — Update main and create release branch

```bash
git checkout main
git pull origin main
git checkout -b release/<version>
```

#### Step 3 — Find changes since last release

Gather PR information for release notes. Find the date of the previous tag (or use repo creation date if first release):

```bash
PREV_TAG=$(git tag --sort=-v:refname | head -1)
if [ -n "$PREV_TAG" ]; then
  SINCE=$(git log -1 --format=%aI "$PREV_TAG")
else
  SINCE=$(git log --reverse --format=%aI | head -1)
fi
```

List merged PRs since that date:

```bash
gh pr list --state merged --search "merged:>$SINCE" --json number,title,author,labels
```

#### Step 4 — Find external contributors

Exclude the maintainer and bots:

```bash
gh pr list --state merged --search "merged:>$SINCE" \
  --json number,title,author \
  --jq '.[] | select(.author.login != "d-kuro" and (.author.login | test("\\[bot\\]$") | not)) | "- @\(.author.login) (#\(.number))"'
```

If no external contributors, omit the Contributors section from the release notes.

#### Step 5 — Write release notes

Create `docs/release-notes/<version>.md` using the template below. Categorize PRs by their content. Omit empty sections.

```markdown
# Release <version>

## 🎉 New Features

### Feature Name (#PR)

Description of the feature.

## 🐛 Bug Fixes

- Fix description (#PR)

## 📚 Documentation

- Documentation changes (#PR)

## 🧹 Code Improvements

- Refactoring or internal changes (#PR)

## ⚠️ Breaking Changes

- Breaking change description

## 👥 Contributors

- @username (#PR)

## 📦 Upgrade Instructions

**Homebrew:**

\`\`\`bash
brew upgrade d-kuro/tap/gwq
\`\`\`

**Go:**

\`\`\`bash
go install github.com/d-kuro/gwq/cmd/gwq@<version>
\`\`\`

**Full Changelog**: https://github.com/d-kuro/gwq/compare/<prev-version>...<version>
```

After writing, show the draft to the user and ask for edits before proceeding.

#### Step 6 — Commit, push, and create PR

```bash
git add docs/release-notes/<version>.md
git commit -m "docs: add release notes for <version>"
git push -u origin release/<version>
```

```bash
gh pr create --title "Release <version>" --body "$(cat <<'EOF'
## Summary

Release <version>

See [docs/release-notes/<version>.md](docs/release-notes/<version>.md) for details.
EOF
)"
```

Tell the user: "PR created. Please merge it once CI passes. After merging, run `/release-gwq <version>` again or ask me to proceed with Phase 2."

### Phase 2: Tag and Publish

This phase runs after the PR is merged. If the user re-invokes the skill with the same version and the PR is already merged, proceed directly here.

#### Step 7 — Create and push tag

```bash
git checkout main
git pull origin main
git tag <version>
git push origin <version>
```

Tell the user: "Tag pushed. The GitHub Actions goreleaser workflow will automatically build and publish the release."

Provide a link to monitor: `https://github.com/d-kuro/gwq/actions/workflows/goreleaser.yaml`

#### Step 8 — Monitor workflow and sync notes

After pushing the tag, use `CronCreate` to poll the release workflow every 2 minutes until it completes:

```
CronCreate(
  cron: "*/2 * * * *",
  recurring: true,
  prompt: "Check the gwq release workflow status by running: gh run list --workflow=goreleaser.yaml --limit 1 --json status,conclusion,databaseId. If the latest run's status is 'completed', delete this cron job with CronDelete, then: if conclusion is 'success', proceed to sync release notes and verify (Step 9). If conclusion is 'failure', report the failure to the user with a link to the run."
)
```

Tell the user: "Monitoring the release workflow. I'll notify you when it completes."

#### Step 9 — Sync notes and verify

Once the workflow succeeds, sync the release notes and verify:

```bash
gh release edit <version> --notes-file docs/release-notes/<version>.md
gh release view <version>
```

Tell the user the release is complete and provide install instructions:

```
brew install d-kuro/tap/gwq
# or
brew upgrade d-kuro/tap/gwq
```

## Phase Detection

When invoked, detect which phase to start from:

1. If on a `release/<version>` branch with uncommitted release notes → resume at Step 5
2. If a PR for `release/<version>` exists and is merged → start Phase 2 (Step 7)
3. If a PR for `release/<version>` exists and is open → tell user to merge, then Phase 2
4. If the tag already exists → skip to Step 8 (sync notes)
5. Otherwise → start from Step 1
