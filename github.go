package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v33/github"
	"golang.org/x/oauth2"
)

var ErrNotMergeable = errors.New("PR not mergeable")
var ErrConflict = errors.New("PR has conflicts")
var ErrBehind = errors.New("PR is behind base branch")

type authenticatedGitHubClient struct {
	ctx    context.Context
	client *github.Client
}

func newAuthenticatedClient(token string) *authenticatedGitHubClient {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return &authenticatedGitHubClient{ctx, client}
}

func (c *authenticatedGitHubClient) updatePRBranch(pr *github.PullRequest) error {
	_, response, err := c.client.PullRequests.UpdateBranch(
		c.ctx,
		pr.Base.Repo.Owner.GetLogin(),
		pr.Base.Repo.GetName(),
		pr.GetNumber(),
		&github.PullRequestBranchUpdateOptions{},
	)

	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status %v when updating branch %v", response.Status, pr.Head.Label)
	}

	return nil
}

func (c *authenticatedGitHubClient) mergePR(pr *github.PullRequest, mergeMethod string) error {
	state := pr.GetMergeableState()

	if strings.EqualFold(state, "dirty") {
		return ErrConflict
	}

	if strings.EqualFold(state, "behind") {
		return ErrBehind
	}

	if !pr.GetMergeable() {
		return ErrNotMergeable
	}

	options := &github.PullRequestOptions{
		MergeMethod: strings.ToLower(mergeMethod),
	}

	result, _, err := c.client.PullRequests.Merge(
		c.ctx,
		pr.Base.Repo.Owner.GetLogin(),
		pr.Base.Repo.GetName(),
		pr.GetNumber(),
		"",
		options,
	)

	if err != nil {
		return err
	}

	if !result.GetMerged() {
		return fmt.Errorf("PR was not merged: %v", result.GetMessage())
	}

	log.Print(result.GetMessage())
	return nil
}
