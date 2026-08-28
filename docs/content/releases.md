---
title: Releases
description: Version and publish Osmose releases with Release Please.
group: Reference
order: 3
layout: doc
---

## Release model

Osmose uses [Release Please](https://github.com/googleapis/release-please) with
Conventional Commits. The workflow runs after changes reach `main`:

1. Release Please creates or updates a release pull request.
2. The release pull request updates the generated `CHANGELOG.md` and release
   metadata.
3. Merging that pull request creates a Git tag and a GitHub Release.

The module is released from the repository root with normal Go module tags such
as `v0.1.0`.

## First release

The repository starts at `0.1.0`. Bootstrap that version once after the release
workflow is merged and `main` is ready:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Then open the new tag in GitHub, choose **Create release from tag**, generate
the release notes, and publish it. The version is tracked in
`.release-please-manifest.json`, so future release pull requests start from
`0.1.0`.

## Future releases

Use a Conventional Commit in each change:

| Prefix | Version effect |
| --- | --- |
| `fix:` | Patch release, such as `0.1.0` → `0.1.1` |
| `feat:` | Minor release, such as `0.1.0` → `0.2.0` |
| `feat!:` or a `BREAKING CHANGE:` footer | Breaking release according to SemVer |

After the change is merged:

1. Wait for the Release Please pull request.
2. Review the version and generated changelog.
3. Merge the release pull request.
4. Verify the GitHub Release and tag.

Consumers can then update the SDK normally:

```bash
go get github.com/ofabiodev/osmose@v0.1.0
```

The workflow uses the repository's `GITHUB_TOKEN`; no npm token or separate
package registry is involved. Repository Actions settings must allow GitHub
Actions to create and approve pull requests.
