# Release Process

This document describes the release process for gwq.

## Overview

gwq uses [GoReleaser](https://goreleaser.com/) for automated releases. When a version tag is pushed, GitHub Actions automatically builds multi-platform binaries and publishes them to GitHub Releases.

## Prerequisites

- Write access to the repository
- `gh` CLI installed and authenticated

## Release Steps

### 1. Update main branch

```bash
git checkout main
git pull origin main
```

### 2. Create release branch

```bash
git checkout -b release/v0.0.X
```

### 3. Create release notes

Create a new file `docs/release-notes/v0.0.X.md` following this template:

```markdown
# Release v0.0.X

## 🎉 New Features

### Feature Name (#PR)

Description of the feature.

**Usage:**
\`\`\`bash
gwq command example
\`\`\`

## 🐛 Bug Fixes

- Fix description (#PR)

## 📚 Documentation

- Documentation changes (#PR)

## 🧹 Code Improvements

- Refactoring or internal changes

## ⚠️ Breaking Changes

- Breaking change description

## 👥 Contributors

- @username (#PR)

## 📦 Upgrade Instructions

\`\`\`bash
go install github.com/d-kuro/gwq/cmd/gwq@v0.0.X
\`\`\`

**Full Changelog**: https://github.com/d-kuro/gwq/compare/v0.0.PREV...v0.0.X
```

#### Contributors section

List external contributors (non-maintainer) who authored PRs included in this release. Bot accounts (renovate, dependabot) should be excluded.

To find contributors, check PR authors between the previous tag and the current release:

```bash
gh pr list --state merged --search "merged:>YYYY-MM-DD" --json number,title,author --jq '.[] | select(.author.login != "d-kuro" and (.author.login | test("\\[bot\\]$") | not)) | "- @\(.author.login) (#\(.number))"'
```

If there are no external contributors, omit the Contributors section entirely.

### 4. Commit and push

```bash
git add docs/release-notes/v0.0.X.md
git commit -m "docs: add release notes for v0.0.X"
git push -u origin release/v0.0.X
```

### 5. Create and merge PR

```bash
gh pr create --title "Release v0.0.X" --body "Release v0.0.X

See [docs/release-notes/v0.0.X.md](docs/release-notes/v0.0.X.md) for details."
```

Merge the PR after CI passes.

### 6. Create and push tag

```bash
git checkout main
git pull origin main
git tag v0.0.X
git push origin v0.0.X
```

### 7. Update GitHub Release notes

After GoReleaser creates the release, sync the release notes:

```bash
gh release edit v0.0.X --notes-file docs/release-notes/v0.0.X.md
```

## Automated Release Process

When a tag is pushed, the following happens automatically:

1. **build.yaml** - Runs tests and linting
2. **goreleaser.yaml** - Builds and publishes release
   - Builds binaries for Linux, macOS, and Windows (amd64, arm64)
   - Creates archives (tar.gz for Unix, zip for Windows)
   - Publishes to GitHub Releases

## Version Scheme

gwq follows [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes and documentation

Current stage: `0.0.X` (initial development)

## Release Checklist

- [ ] All tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Release notes created in `docs/release-notes/`
- [ ] Release PR merged
- [ ] Tag created and pushed
- [ ] GitHub Actions completed successfully
- [ ] GitHub Release notes synced with markdown file

## Troubleshooting

### GoReleaser failed

Check the GitHub Actions log for errors. Common issues:

- Missing or invalid `.goreleaser.yaml`
- Build errors in the code

### Release notes not showing

Ensure you ran:

```bash
gh release edit v0.0.X --notes-file docs/release-notes/v0.0.X.md
```
