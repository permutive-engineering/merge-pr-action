package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/go-github/v33/github"
)

const (
	eventNameVariable     = "GITHUB_EVENT_NAME"
	payloadPathVariable   = "GITHUB_EVENT_PATH"
	tokenVariable         = "INPUT_GITHUB_TOKEN"
	allowedUpdateVariable = "INPUT_ALLOWED_UPDATE"
	mergeMethodVariable   = "INPUT_MERGE_METHOD"
)

type pullRequestEvent struct {
	PullRequest github.PullRequest `json:"pull_request"`
}

func getRequiredEnvVar(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("required env variable %v not set", name)
	}

	return value
}

func main() {
	if eventName := getRequiredEnvVar(eventNameVariable); eventName != "pull_request" {
		log.Println("event is not `pull_request`, exiting")
		os.Exit(0)
		return
	}

	payloadPath := getRequiredEnvVar(payloadPathVariable)

	payload, err := ioutil.ReadFile(payloadPath)
	if err != nil {
		log.Fatalf("error opening %v: %v", payloadPath, err.Error())
	}

	var event pullRequestEvent
	err = json.Unmarshal(payload, &event)
	if err != nil {
		log.Fatalf("error parsing event JSON: %v", err.Error())
	}

	if event.PullRequest.Title == nil {
		log.Fatalf("no pull request title in event payload")
	}

	upgrade, err := parseVersionUpgrade(*event.PullRequest.Title)
	if err != nil {
		log.Fatalf("error parsing upgrade from PR title %v: %v", event.PullRequest.Title, err.Error())
	}
	upgradeType := upgrade.UpgradeType()

	log.Printf("detected upgrade: %v", upgrade)

	allowedUpgrade, err := parseUpgradeType(os.Getenv(allowedUpdateVariable))
	if err != nil {
		log.Fatalf("error parsing allowed upgrade type: %v", err.Error())
	}

	if !allowed(allowedUpgrade, upgradeType) {
		log.Printf("%v upgrade not allowed, skipping", upgradeType)
		os.Exit(0)
	}

	token := getRequiredEnvVar(tokenVariable)
	mergeMethod := getRequiredEnvVar(mergeMethodVariable)
	client := newAuthenticatedClient(token)

	if err := client.mergePR(&event.PullRequest, mergeMethod, 0); err != nil {
		// if the PR branch is behind the base, trigger an update (merge the base branch into the PR)
		// this merge should retrigger the workflow that runs this action
		// see https://docs.github.com/en/rest/reference/pulls#update-a-pull-request-branch
		if errors.Is(err, ErrBehind) {
			log.Print("branch is behind base, updating")

			if err := client.updatePRBranch(&event.PullRequest); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatalf("error merging PR: %v", err.Error())
		}
	}

	os.Exit(0)
}
