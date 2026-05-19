package client

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/caarlos0/log"
	"github.com/goreleaser/goreleaser/v2/internal/artifact"
	"github.com/goreleaser/goreleaser/v2/internal/changelog"
	"github.com/goreleaser/goreleaser/v2/internal/retryx"
	"github.com/goreleaser/goreleaser/v2/internal/tmpl"
	"github.com/goreleaser/goreleaser/v2/pkg/config"
	"github.com/goreleaser/goreleaser/v2/pkg/context"
	gitee "github.com/next-bin/go-gitee/gitee"
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
		release, err = c.updateRelease(ctx, title, body, int64(*release.ID))
		if err != nil {
			return "", err
		}
	} else {
		release, err = c.createRelease(ctx, title, body)
		if err != nil {
			return "", err
		}
	}

	return strconv.FormatInt(int64(*release.ID), 10), nil
}

func (c *giteeClient) PublishRelease(_ *context.Context, _ string) error {
	return nil
}

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

func (c *giteeClient) Changelog(ctx *context.Context, repo Repo, prev, current string) ([]ChangelogItem, error) {
	compare, _, err := giteeDo(ctx, func() (*gitee.Compare, *gitee.Response, error) {
		return c.client.Repositories.CompareCommits(ctx, repo.Owner, repo.Name, prev, current, nil)
	})
	if err != nil {
		return nil, err
	}

	var logResult []ChangelogItem
	commits := compare.Commits
	if commits == nil {
		return logResult, nil
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

		logResult = append(logResult, fillDeprecated(item))
	}
	return logResult, nil
}

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

	existing, resp, err := giteeDo(ctx, func() ([]*gitee.Content, *gitee.Response, error) {
		return c.client.Repositories.GetContents(ctx, owner, repoName, path, nil)
	})
	if err != nil {
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			return err
		}
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
