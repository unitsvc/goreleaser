# Gitee Release Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Gitee (gitee.com) as a fourth release platform in goreleaser using the go-gitee SDK.

**Architecture:** Add a `TokenTypeGitee` constant, a `giteeClient` implementing the existing `Client` interface, and wire it into the env detection, config parsing, and release pipe — following the exact same pattern as Gitea.

**Tech Stack:** Go, goreleaser v2, github.com/next-bin/go-gitee/gitee SDK

---

### Task 1: Add dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add go-gitee dependency**

Run:
```bash
cd goreleaser && go get github.com/next-bin/go-gitee/gitee
```
Expected: `go.mod` and `go.sum` updated with the new dependency.

- [ ] **Step 2: Verify module resolves**

Run:
```bash
cd goreleaser && go mod tidy
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add go-gitee SDK dependency"
```

---

### Task 2: Add TokenType constant

**Files:**
- Modify: `pkg/context/context.go:62-69`

- [ ] **Step 1: Add the constant**

After line 68 (`TokenTypeGitea TokenType = "gitea"`), add:

```go
// TokenTypeGitee defines gitee as type of the token.
TokenTypeGitee TokenType = "gitee"
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./pkg/context/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add pkg/context/context.go
git commit -m "feat: add TokenTypeGitee constant"
```

---

### Task 3: Add config structs and fields

**Files:**
- Modify: `pkg/config/config.go:42-46` (add GiteeURLs after GiteaURLs)
- Modify: `pkg/config/config.go:649` (add Gitee Repo to Release)
- Modify: `pkg/config/config.go:1187` (add GiteeToken to EnvFiles)
- Modify: `pkg/config/config.go:1326` (add GiteeURLs to Project)

- [ ] **Step 1: Add GiteeURLs struct**

After line 46 (closing brace of `GiteaURLs`), add:

```go
// GiteeURLs holds the URLs to be used when using gitee.
type GiteeURLs struct {
	API           string `yaml:"api,omitempty" json:"api,omitempty"`
	Download      string `yaml:"download,omitempty" json:"download,omitempty"`
	SkipTLSVerify bool   `yaml:"skip_tls_verify,omitempty" json:"skip_tls_verify,omitempty"`
}
```

- [ ] **Step 2: Add Gitee field to Release struct**

After line 649 (`Gitea Repo ...`), add:

```go
	Gitee                  Repo        `yaml:"gitee,omitempty" json:"gitee,omitempty"`
```

- [ ] **Step 3: Add GiteeToken to EnvFiles**

After line 1187 (`GiteaToken string ...`), add:

```go
	GiteeToken  string `yaml:"gitee_token,omitempty" json:"gitee_token,omitempty"`
```

- [ ] **Step 4: Add GiteeURLs to Project struct**

After line 1326 (`GiteaURLs GiteaURLs ...`), add:

```go
	// should be set if using Gitee
	GiteeURLs GiteeURLs `yaml:"gitee_urls,omitempty" json:"gitee_urls,omitempty"`
```

- [ ] **Step 5: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./pkg/config/...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add pkg/config/config.go
git commit -m "feat: add GiteeURLs, Release.Gitee, EnvFiles.GiteeToken config fields"
```

---

### Task 4: Add Gitee client — constructor and helper

**Files:**
- Create: `internal/client/gitee.go`

- [ ] **Step 1: Write gitee.go with constructor and retry helper**

Create `internal/client/gitee.go`:

```go
package client

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	gitee "github.com/next-bin/go-gitee/gitee"
	"github.com/caarlos0/log"
	"github.com/goreleaser/goreleaser/v2/internal/artifact"
	"github.com/goreleaser/goreleaser/v2/internal/changelog"
	"github.com/goreleaser/goreleaser/v2/internal/retryx"
	"github.com/goreleaser/goreleaser/v2/internal/tmpl"
	"github.com/goreleaser/goreleaser/v2/pkg/config"
	"github.com/goreleaser/goreleaser/v2/pkg/context"
)

type giteeClient struct {
	client *gitee.Client
}

var _ Client = &giteeClient{}

func giteeDo[T any](ctx *context.Context, fn func() (T, *gitee.Response, error)) (T, *gitee.Response, error) {
	var result T
	var resp *gitee.Response
	err := retryx.Do(ctx, ctx.Config.Retry, func() error {
		var err error
		result, resp, err = fn()
		if err != nil {
			return retryx.HTTP(err, must(resp).Response)
		}
		return nil
	}, retryx.IsRetriable)
	return result, resp, err
}

func newGitee(ctx *context.Context, token string) (*giteeClient, error) {
	apiURL, err := tmpl.New(ctx).Apply(ctx.Config.GiteeURLs.API)
	if err != nil {
		return nil, fmt.Errorf("templating Gitee API URL: %w", err)
	}
	if apiURL == "" {
		apiURL = "https://gitee.com/api/v5/"
	}

	opts := []gitee.ClientOptionsFunc{
		gitee.WithToken(token),
		gitee.WithBaseURL(apiURL),
	}
	client, err := gitee.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &giteeClient{client: client}, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./internal/client/...
```
Expected: compilation may fail until Task 5 wires the factory — but the file itself should parse.

- [ ] **Step 3: Commit**

```bash
git add internal/client/gitee.go
git commit -m "feat: add giteeClient constructor and retry helper"
```

---

### Task 5: Wire Gitee into client factory

**Files:**
- Modify: `internal/client/client.go:136-148`

- [ ] **Step 1: Add Gitee case to newWithToken**

In `internal/client/client.go`, add a new case after line 144 (`case context.TokenTypeGitea:`):

```go
	case context.TokenTypeGitee:
		return newGitee(ctx, token)
```

The switch in `newWithToken` becomes:

```go
func newWithToken(ctx *context.Context, token string) (Client, error) {
	log.WithField("type", ctx.TokenType).Debug("token type")
	switch ctx.TokenType {
	case context.TokenTypeGitHub:
		return newGitHub(ctx, token)
	case context.TokenTypeGitLab:
		return newGitLab(ctx, token)
	case context.TokenTypeGitea:
		return newGitea(ctx, token)
	case context.TokenTypeGitee:
		return newGitee(ctx, token)
	default:
		return nil, fmt.Errorf("invalid client token type: %q", ctx.TokenType)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./internal/client/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/client/client.go
git commit -m "feat: wire giteeClient into client factory"
```

---

### Task 6: Implement giteeClient — CreateRelease, PublishRelease, Upload

**Files:**
- Modify: `internal/client/gitee.go`

- [ ] **Step 1: Add helper methods for release CRUD**

Append to `gitee.go`:

```go
func (c *giteeClient) createRelease(ctx *context.Context, title, body string) (*gitee.Release, error) {
	releaseConfig := ctx.Config.Release
	owner := releaseConfig.Gitee.Owner
	repoName := releaseConfig.Gitee.Name

	opts := &gitee.CreateReleaseOptions{
		TagName:         gitee.Ptr(ctx.Git.CurrentTag),
		Name:            gitee.Ptr(title),
		Body:            gitee.Ptr(body),
		TargetCommitish: gitee.Ptr(ctx.Git.Commit),
		Prerelease:      gitee.Ptr(ctx.PreRelease),
	}

	release, _, err := giteeDo(ctx, func() (*gitee.Release, *gitee.Response, error) {
		return c.client.Repositories.CreateRelease(ctx, owner, repoName, opts)
	})
	if err != nil {
		log.WithError(err).Debug("error creating Gitee release")
		return nil, err
	}
	log.WithField("id", *release.ID).Info("Gitee release created")
	return release, nil
}

func (c *giteeClient) getExistingRelease(ctx *context.Context, owner, repoName, tagName string) (*gitee.Release, error) {
	release, resp, err := giteeDo(ctx, func() (*gitee.Release, *gitee.Response, error) {
		return c.client.Repositories.GetReleaseByTag(ctx, owner, repoName, tagName)
	})
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return release, nil
}

func (c *giteeClient) updateRelease(ctx *context.Context, title, body string, id int64) (*gitee.Release, error) {
	releaseConfig := ctx.Config.Release
	owner := releaseConfig.Gitee.Owner
	repoName := releaseConfig.Gitee.Name

	opts := &gitee.UpdateReleaseOptions{
		TagName:    gitee.Ptr(ctx.Git.CurrentTag),
		Name:       gitee.Ptr(title),
		Body:       gitee.Ptr(body),
		Prerelease: gitee.Ptr(ctx.PreRelease),
	}

	release, _, err := giteeDo(ctx, func() (*gitee.Release, *gitee.Response, error) {
		return c.client.Repositories.UpdateRelease(ctx, owner, repoName, id, opts)
	})
	if err != nil {
		log.WithError(err).Debug("error updating Gitee release")
		return nil, err
	}
	log.WithField("id", *release.ID).Info("Gitee release updated")
	return release, nil
}

// CreateRelease creates a new release or updates it if it already exists.
func (c *giteeClient) CreateRelease(ctx *context.Context, body string) (string, error) {
	releaseConfig := ctx.Config.Release

	title, err := tmpl.New(ctx).Apply(releaseConfig.NameTemplate)
	if err != nil {
		return "", err
	}

	release, err := c.getExistingRelease(
		ctx,
		releaseConfig.Gitee.Owner,
		releaseConfig.Gitee.Name,
		ctx.Git.CurrentTag,
	)
	if err != nil {
		return "", err
	}

	if release != nil {
		body = getReleaseNotes(*release.Body, body, ctx.Config.Release.ReleaseNotesMode)
		release, err = c.updateRelease(ctx, title, body, *release.ID)
		if err != nil {
			return "", err
		}
	} else {
		release, err = c.createRelease(ctx, title, body)
		if err != nil {
			return "", err
		}
	}

	return strconv.FormatInt(*release.ID, 10), nil
}

// PublishRelease marks the release as published.
func (c *giteeClient) PublishRelease(_ *context.Context, _ string) error {
	// Gitee does not have a draft/publish flow — release is published on creation.
	return nil
}

// Upload uploads a file into a release repository.
func (c *giteeClient) Upload(
	ctx *context.Context,
	releaseID string,
	art *artifact.Artifact,
) error {
	giteeReleaseID, err := strconv.ParseInt(releaseID, 10, 64)
	if err != nil {
		return err
	}

	releaseConfig := ctx.Config.Release
	owner := releaseConfig.Gitee.Owner
	repoName := releaseConfig.Gitee.Name

	return retryx.Do(ctx, ctx.Config.Retry, func() error {
		file, err := os.Open(art.Path)
		if err != nil {
			return retryx.Unrecoverable(err)
		}
		defer file.Close()

		_, resp, err := c.client.Repositories.UploadReleaseAttachFile(ctx, owner, repoName, giteeReleaseID, art.Name, file)
		return retryx.HTTP(err, must(resp).Response)
	}, retryx.IsRetriable)
}
```

**Note:** `gitee.Ptr` is the go-gitee SDK helper for creating pointer values. Verify it exists — if not, use a local helper.

- [ ] **Step 2: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./internal/client/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/client/gitee.go
git commit -m "feat: implement giteeClient CreateRelease, PublishRelease, Upload"
```

---

### Task 7: Implement giteeClient — Changelog, CloseMilestone, CreateFile, ReleaseURLTemplate

**Files:**
- Modify: `internal/client/gitee.go`

- [ ] **Step 1: Add remaining interface methods**

Append to `gitee.go`:

```go
// Changelog fetches the changelog between two revisions.
func (c *giteeClient) Changelog(ctx *context.Context, repo Repo, prev, current string) ([]ChangelogItem, error) {
	compare, _, err := giteeDo(ctx, func() (*gitee.Compare, *gitee.Response, error) {
		return c.client.Repositories.CompareCommits(ctx, repo.Owner, repo.Name, prev, current, nil)
	})
	if err != nil {
		return nil, err
	}

	var log []ChangelogItem
	commits := compare.Commits
	if commits == nil {
		return log, nil
	}

	for _, commit := range *commits {
		item := ChangelogItem{
			SHA: *commit.Sha,
		}
		if commit.Commit != nil && commit.Commit.Message != nil {
			item.Message = strings.Split(*commit.Commit.Message, "\n")[0]
		}

		if commit.Commit != nil && commit.Commit.Author != nil {
			author := commit.Commit.Author
			name := ""
			email := ""
			if author.Name != nil {
				name = *author.Name
			}
			if author.Email != nil {
				email = *author.Email
			}
			item.Authors = append(item.Authors, Author{
				Name:  name,
				Email: email,
			})
		}

		if commit.Commit != nil && commit.Commit.Message != nil {
			item.Authors = append(item.Authors, changelog.ExtractCoAuthors(*commit.Commit.Message)...)
		}

		log = append(log, fillDeprecated(item))
	}
	return log, nil
}

// CloseMilestone closes a given milestone by title.
func (c *giteeClient) CloseMilestone(ctx *context.Context, repo Repo, title string) error {
	milestones, _, err := giteeDo(ctx, func() ([]*gitee.Milestone, *gitee.Response, error) {
		return c.client.Milestones.List(ctx, repo.Owner, repo.Name, nil)
	})
	if err != nil {
		return err
	}

	for _, m := range milestones {
		if m.Title != nil && *m.Title == title {
			closed := "closed"
			_, _, err := giteeDo(ctx, func() (*gitee.Milestone, *gitee.Response, error) {
				return c.client.Milestones.Edit(ctx, repo.Owner, repo.Name, *m.Number, &gitee.UpdateMilestoneOptions{
					State: &closed,
				})
			})
			return err
		}
	}

	return ErrNoMilestoneFound{Title: title}
}

// CreateFile creates or updates a file in the repository.
func (c *giteeClient) CreateFile(
	ctx *context.Context,
	commitAuthor config.CommitAuthor,
	repo Repo,
	content []byte,
	path,
	message string,
) error {
	owner := repo.Owner
	repoName := repo.Name

	// Check if file already exists
	existing, resp, err := giteeDo(ctx, func() ([]*gitee.Content, *gitee.Response, error) {
		return c.client.Repositories.GetContents(ctx, owner, repoName, path, nil)
	})
	if err != nil {
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			return err
		}
		// File doesn't exist — create it
		encoded := base64.StdEncoding.EncodeToString(content)
		_, _, err = giteeDo(ctx, func() (*gitee.CommitContent, *gitee.Response, error) {
			return c.client.Repositories.CreateFile(ctx, owner, repoName, path, &gitee.CreateContentOptions{
				Message:        gitee.Ptr(message),
				Content:        &encoded,
				CommitterName:  gitee.Ptr(commitAuthor.Name),
				CommitterEmail: gitee.Ptr(commitAuthor.Email),
				AuthorName:     gitee.Ptr(commitAuthor.Name),
				AuthorEmail:    gitee.Ptr(commitAuthor.Email),
			})
		})
		return err
	}

	// File exists — update it
	var sha string
	if len(existing) > 0 && existing[0].Sha != nil {
		sha = *existing[0].Sha
	}
	encoded := base64.StdEncoding.EncodeToString(content)
	_, _, err = giteeDo(ctx, func() (*gitee.CommitContent, *gitee.Response, error) {
		return c.client.Repositories.UpdateFile(ctx, owner, repoName, path, &gitee.UpdateContentOptions{
			Message:        gitee.Ptr(message),
			Content:        &encoded,
			SHA:            gitee.Ptr(sha),
			CommitterName:  gitee.Ptr(commitAuthor.Name),
			CommitterEmail: gitee.Ptr(commitAuthor.Email),
			AuthorName:     gitee.Ptr(commitAuthor.Name),
			AuthorEmail:    gitee.Ptr(commitAuthor.Email),
		})
	})
	return err
}

// ReleaseURLTemplate returns the download URL template for Gitee releases.
func (c *giteeClient) ReleaseURLTemplate(ctx *context.Context) (string, error) {
	downloadURL, err := tmpl.New(ctx).Apply(ctx.Config.GiteeURLs.Download)
	if err != nil {
		return "", fmt.Errorf("templating Gitee download URL: %w", err)
	}

	return fmt.Sprintf(
		"%s%s/%s/releases/download/{{ urlPathEscape .Tag }}/{{ .ArtifactName }}",
		downloadURL,
		ctx.Config.Release.Gitee.Owner,
		ctx.Config.Release.Gitee.Name,
	), nil
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./internal/client/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/client/gitee.go
git commit -m "feat: implement giteeClient Changelog, CloseMilestone, CreateFile, ReleaseURLTemplate"
```

---

### Task 8: Add GITEE_TOKEN env detection

**Files:**
- Modify: `internal/pipe/env/env.go`

- [ ] **Step 1: Update ErrMissingToken message**

Line 23 — change:

```go
var ErrMissingToken = errors.New("missing GITHUB_TOKEN, GITLAB_TOKEN, GITEA_TOKEN and GITEE_TOKEN")
```

- [ ] **Step 2: Add default token file**

In `setDefaultTokenFiles` (line 42), after line 52 (`env.GiteaToken = ...`), add:

```go
	if env.GiteeToken == "" {
		env.GiteeToken = "~/.config/goreleaser/gitee_token"
	}
```

- [ ] **Step 3: Add GITEE_TOKEN loading**

After line 71 (`giteaToken, giteaTokenErr := ...`), add:

```go
	giteeToken, giteeTokenErr := loadEnv("GITEE_TOKEN", ctx.Config.EnvFiles.GiteeToken)
```

- [ ] **Step 4: Add GITEE_TOKEN to ForceToken switch**

In the `switch strings.ToLower(forceToken)` block, add a new case after `case "gitea":`:

```go
		case "gitee":
			githubToken = ""
			gitlabToken = ""
			giteaToken = ""
```

And update the existing cases to also clear `giteeToken`:

```go
		case "github":
			gitlabToken = ""
			giteaToken = ""
			giteeToken = ""
		case "gitlab":
			githubToken = ""
			giteaToken = ""
			giteeToken = ""
		case "gitea":
			githubToken = ""
			gitlabToken = ""
			giteeToken = ""
```

- [ ] **Step 5: Add GITEE_TOKEN to multiple-tokens check**

In the `default:` branch, after the `giteaToken` block (line 96), add:

```go
			if giteeToken != "" {
				tokens = append(tokens, "GITEE_TOKEN")
			}
```

- [ ] **Step 6: Update noTokens and noTokenErrs checks**

Line 103 — change to:

```go
	noTokens := githubToken == "" && gitlabToken == "" && giteaToken == "" && giteeToken == ""
	noTokenErrs := githubTokenErr == nil && gitlabTokenErr == nil && giteaTokenErr == nil && giteeTokenErr == nil
```

- [ ] **Step 7: Update checkErrors call**

Add `giteeTokenErr` as a parameter to `checkErrors`. Update the function signature and body:

```go
	if err := checkErrors(ctx, noTokens, noTokenErrs, gitlabTokenErr, githubTokenErr, giteaTokenErr, giteeTokenErr); err != nil {
```

And update the `checkErrors` function signature and body:

```go
func checkErrors(ctx *context.Context, noTokens, noTokenErrs bool, gitlabTokenErr, githubTokenErr, giteaTokenErr, giteeTokenErr error) error {
```

Add before the final `return nil`:

```go
	if giteeTokenErr != nil {
		return fmt.Errorf("failed to load gitee token: %w", giteeTokenErr)
	}
```

- [ ] **Step 8: Add Gitee token type detection**

After the `giteaToken` detection block (lines 116-120), add:

```go
	if giteeToken != "" {
		log.Debug("token type: gitee")
		ctx.TokenType = context.TokenTypeGitee
		ctx.Token = giteeToken
	}
```

This must come **after** the gitea block so Gitee has higher priority (last one wins in the token chain).

- [ ] **Step 9: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./internal/pipe/env/...
```
Expected: no errors.

- [ ] **Step 10: Commit**

```bash
git add internal/pipe/env/env.go
git commit -m "feat: add GITEE_TOKEN env detection"
```

---

### Task 9: Wire Gitee into release pipe

**Files:**
- Modify: `internal/pipe/release/release.go:39-48,57-71,116-124`

- [ ] **Step 1: Add Gitee release counting**

After line 48 (`if ctx.Config.Release.Gitea.String() != ""`), add:

```go
		if ctx.Config.Release.Gitee.String() != "" {
			numOfReleases++
		}
```

- [ ] **Step 2: Add Gitee case to Default() switch**

After line 63 (`case context.TokenTypeGitea:`), add:

```go
		case context.TokenTypeGitee:
			if err := setupGitee(ctx); err != nil {
				return err
			}
```

- [ ] **Step 3: Add Gitee case to releaseRepo()**

After line 120 (`case context.TokenTypeGitea:`), add:

```go
		case context.TokenTypeGitee:
			return ctx.Config.Release.Gitee
```

- [ ] **Step 4: Verify it compiles**

Run:
```bash
cd goreleaser && go build ./internal/pipe/release/...
```
Expected: compilation will fail because `setupGitee` doesn't exist yet — that's Task 10.

---

### Task 10: Add setupGitee to scm.go

**Files:**
- Modify: `internal/pipe/release/scm.go` (append after line 89)

- [ ] **Step 1: Add setupGitee function**

Append to `scm.go`:

```go
func setupGitee(ctx *context.Context) error {
	if ctx.Config.Release.Gitee.Name == "" {
		repo, err := getRepository(ctx)
		if err != nil {
			return err
		}
		ctx.Config.Release.Gitee = repo
	}

	if err := tmpl.New(ctx).ApplyAll(
		&ctx.Config.Release.Gitee.Name,
		&ctx.Config.Release.Gitee.Owner,
	); err != nil {
		return err
	}

	// Set default URLs
	if ctx.Config.GiteeURLs.API == "" {
		ctx.Config.GiteeURLs.API = "https://gitee.com/api/v5/"
	}
	if ctx.Config.GiteeURLs.Download == "" {
		ctx.Config.GiteeURLs.Download = "https://gitee.com/"
	}

	url, err := tmpl.New(ctx).Apply(fmt.Sprintf(
		"%s/%s/%s/releases/tag/%s",
		ctx.Config.GiteeURLs.Download,
		ctx.Config.Release.Gitee.Owner,
		ctx.Config.Release.Gitee.Name,
		ctx.Git.CurrentTag,
	))
	ctx.ReleaseURL = url
	return err
}
```

Also add `"fmt"` to the imports if not already present.

- [ ] **Step 2: Verify full build**

Run:
```bash
cd goreleaser && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit Tasks 9 + 10 together**

```bash
git add internal/pipe/release/release.go internal/pipe/release/scm.go
git commit -m "feat: wire Gitee into release pipe and add setupGitee"
```

---

### Task 11: Full build and test

**Files:**
- None (verification only)

- [ ] **Step 1: Full build**

Run:
```bash
cd goreleaser && go build ./...
```
Expected: no errors.

- [ ] **Step 2: Run existing tests**

Run:
```bash
cd goreleaser && go test ./internal/client/... -count=1 -timeout 5m
```
Expected: all existing tests pass (no new tests for Gitee yet, but no regressions).

- [ ] **Step 3: Run release pipe tests**

Run:
```bash
cd goreleaser && go test ./internal/pipe/release/... -count=1 -timeout 5m
```
Expected: all existing tests pass.

- [ ] **Step 4: Run env pipe tests**

Run:
```bash
cd goreleaser && go test ./internal/pipe/env/... -count=1 -timeout 5m
```
Expected: all existing tests pass.

---

### Task 12: Final commit (squash if needed)

- [ ] **Step 1: Review all changes**

Run:
```bash
cd goreleaser && git log --oneline feat/gitee-support --not main
```
Expected: list of all commits from Tasks 1-10.

- [ ] **Step 2: Push**

```bash
git push -u origin feat/gitee-support
```
