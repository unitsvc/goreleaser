---
name: Gitee Release Support
type: spec
created: 2026-05-19
status: draft
---

# Gitee Release Support

## Background

goreleaser supports GitHub, GitLab, and Gitea as release targets. Gitee (gitee.com) is a major Chinese Git hosting platform with a RESTful API v5. The `go-gitee` SDK (`github.com/next-bin/go-gitee/gitee`) already provides full API coverage for Gitee releases, milestones, commits, and file contents.

## Goal

Add Gitee as a fourth release platform. Users set `GITEE_TOKEN` to publish. Single-run-single-platform architecture unchanged.

## Configuration

```yaml
release:
  gitee:
    owner: myuser
    name: myrepo

gitee_urls:
  api: https://gitee.com/api/v5/
  download: https://gitee.com/
  skip_tls_verify: false

env_files:
  gitee_token: ~/.config/goreleaser/gitee_token
```

### Defaults

`gitee_urls.api` defaults to `https://gitee.com/api/v5/`.
`gitee_urls.download` defaults to `https://gitee.com/`.
`release.gitee` infers owner/name from git remote when empty.

## File Changes

### `pkg/context/context.go`

Add:

```go
TokenTypeGitee TokenType = "gitee"
```

### `pkg/config/config.go`

Add struct:

```go
type GiteeURLs struct {
    API           string `yaml:"api,omitempty" json:"api,omitempty"`
    Download      string `yaml:"download,omitempty" json:"download,omitempty"`
    SkipTLSVerify bool   `yaml:"skip_tls_verify,omitempty" json:"skip_tls_verify,omitempty"`
}
```

Add fields:
- `Release.Gitee Repo` — `yaml:"gitee,omitempty" json:"gitee,omitempty"`
- `Project.GiteeURLs GiteeURLs` — `yaml:"gitee_urls,omitempty" json:"gitee_urls,omitempty"`
- `EnvFiles.GiteeToken string` — `yaml:"gitee_token,omitempty" json:"gitee_token,omitempty"`

### `internal/client/gitee.go` (new)

Implements `Client` interface via `github.com/next-bin/go-gitee/gitee`.

```go
type giteeClient struct {
    client *gitee.Client
    repo   Repo
    url    GiteeURLs
}

func newGitee(ctx *context.Context, token string) (Client, error)
```

| Method | SDK call | Details |
|--------|----------|---------|
| `CreateRelease` | `RepositoriesService.CreateRelease()` | Check existing by tag first via `GetReleaseByTag()` |
| `PublishRelease` | `RepositoriesService.UpdateRelease()` | Set `prerelease=false` |
| `Upload` | `RepositoriesService.UploadReleaseAttachFile()` | Multipart upload with retry |
| `Changelog` | `RepositoriesService.ListCommits()` | Map `RepoCommit.Sha/.Commit.Message/.Commit.Author` to `ChangelogItem` |
| `CloseMilestone` | `MilestonesService.List()` + `Edit()` | Find by title, set `state=closed` |
| `ReleaseURLTemplate` | Computed from `GiteeURLs.Download` | Format: `{download}{owner}/{repo}/releases/download/{tag}/{name}` |
| `CreateFile` | `RepositoriesService.CreateFile()` / `UpdateFile()` | Check existence first |

### `internal/client/client.go`

Add case:

```go
case context.TokenTypeGitee:
    return newGitee(ctx, token)
```

### `internal/pipe/env/env.go`

Add `GITEE_TOKEN` detection with highest priority in the token chain:

```go
giteeToken, _ := loadEnv("GITEE_TOKEN", ctx.Config.EnvFiles.GiteeToken)
if giteeToken != "" {
    ctx.TokenType = context.TokenTypeGitee
    ctx.Token = giteeToken
}
```

Default token file: `~/.config/goreleaser/gitee_token`.

### `internal/pipe/release/release.go`

Add `TokenTypeGitee` case to `Default()` switch calling `setupGitee()`.

### `internal/pipe/release/scm.go`

Add `setupGitee(ctx)` — mirrors `setupGitea()`:
- Infer repo from git remote if `release.gitee` is empty
- Apply templates to owner/name
- Set `gitee_urls` defaults

## Dependency

```
go get github.com/next-bin/go-gitee/gitee
```

## Testing

- Unit tests for `giteeClient` with mock HTTP server
- Config parsing tests for `release.gitee` and `gitee_urls`
- Env detection tests for `GITEE_TOKEN`
- `setupGitee()` default URL tests

## Limitations

- Single platform per run (unchanged goreleaser behavior)
- Gitee has no native "publish draft" API — `PublishRelease` uses `UpdateRelease`
- `CloseMilestone` requires list+edit since Gitee has no close-by-title API
