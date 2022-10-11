package promoter

import (
	"encoding/json"
	"errors"
	"fmt"
	"keptn/git-promotion-service/pkg/model"
	"keptn/git-promotion-service/pkg/replacer"
	"keptn/git-promotion-service/pkg/repoaccess"
	"strings"

	logger "github.com/sirupsen/logrus"
)

type FlatPrPromoter struct {
	client repoaccess.Client
}

func NewFlatPrPromoter(client repoaccess.Client) FlatPrPromoter {
	return FlatPrPromoter{client: client}
}

func (promoter FlatPrPromoter) Promote(repositoryUrl string, fields map[string]string, sourceBranch, targetBranch, title, body string, paths []model.Path) (message string, prLink *string, err error) {
	logger.WithField("func", "manageFlatPRStrategy").Infof("starting flat pr strategy with sourceBranch %s and targetBranch %s and fields %v", sourceBranch, targetBranch, fields)

	if exists, err := promoter.client.BranchExists(targetBranch); err != nil {
		return "", nil, err
	} else if exists {
		return "", nil, errors.New(fmt.Sprintf("branch with name %s already exists", targetBranch))
	}
	if err := promoter.client.CreateBranch(sourceBranch, targetBranch); err != nil {
		return "", nil, err
	}
	changes := 0
	logger.WithField("func", "manageFlatPRStrategy").Infof("processing %d paths", len(paths))
	for _, p := range paths {
		var path string
		if p.Source == nil {
			path = *p.Target
		} else {
			path = *p.Source
		}

		logger.WithField("func", "manageFlatPRStrategy").Infof("Getting Files for branch %s path %s", sourceBranch, path)
		//LOG: "Getting Files for branch main path /kube-infra/kustomize/podtato-head/podtato-head/envs/dev/version.yaml"
		pNewTargetFiles, err := promoter.client.GetFilesForBranch(sourceBranch, path)
		if err != nil {
			logger.WithField("func", "manageFlatPRStrategy").Infof("Couldnt get files for branch %s path %s", sourceBranch, path)
			return "", nil, err
		}
		var pCurrentTargetFiles []repoaccess.RepositoryFile
		if p.Source != nil {
			if pCurrentTargetFiles, err = promoter.client.GetFilesForBranch(sourceBranch, *p.Target); err != nil {
				logger.WithField("func", "manageFlatPRStrategy").Infof("p.Source != nil and err in GetFilesforBranch branch: %s target %s", sourceBranch, *p.Target)
				return "", nil, err
			}
		} else {
			logger.WithField("func", "manageFlatPRStrategy").Infof("pCurrentTargetFiles %s pNewTargetFiles %s", pCurrentTargetFiles, pNewTargetFiles)
			pCurrentTargetFiles = pNewTargetFiles
		}
		pCurrentTargetFilesJSON, err := json.Marshal(pCurrentTargetFiles)
		pNewTargetFilesJSON, err := json.Marshal(pNewTargetFiles)

		logger.WithField("func", "manageFlatPRStrategy").Infof("pCurrentTargetFilesJSON %s", pCurrentTargetFilesJSON)
		//LOG: "pCurrentTargetFilesJSON null"
		logger.WithField("func", "manageFlatPRStrategy").Infof("pNewTargetFiles %s", pNewTargetFilesJSON)
		//LOG: "pNewTargetFiles [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.7\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/dev/version.yaml\",\"SHA\":\"6f244d800c062740187b1893b2d41551c94ae02a\"}]

		for i, c := range pNewTargetFiles {
			pNewTargetFiles[i].Content = replacer.Replace(c.Content, fields)
			if p.Source != nil {
				pNewTargetFiles[i].Path = strings.Replace(pNewTargetFiles[i].Path, *p.Source, *p.Target, -1)
			}
		}

		changedpNewTargetFilesJSON, err := json.Marshal(pNewTargetFiles)
		logger.WithField("func", "manageFlatPRStrategy").Infof("Modified %s", changedpNewTargetFilesJSON)

		if checkForChanges(pNewTargetFiles, pCurrentTargetFiles) {
			if pathChanges, err := promoter.client.SyncFilesWithBranch(targetBranch, pCurrentTargetFiles, pNewTargetFiles); err != nil {
				return "", nil, err
			} else {
				logger.WithField("func", "manageFlatPRStrategy").Info("There were changes")
				changes += pathChanges
			}
		} else {
			logger.WithField("func", "manageFlatPRStrategy").Info("no changes detected, doing nothing")
			return "no changes detected", nil, nil
		}
	}
	logger.WithField("func", "manageFlatPRStrategy").Infof("commited %d changes to branch %s", changes, targetBranch)
	if changes > 0 {
		if pr, err := promoter.client.CreatePullRequest(targetBranch, sourceBranch, title, body); err != nil {
			return "", nil, err
		} else {
			logger.WithField("func", "manageFlatPRStrategy").Infof("opened pull request %d in repo %s from branch %s to %s", pr.Number, repositoryUrl, sourceBranch, targetBranch)
			return "opened pull request", &pr.URL, nil
		}
	} else {
		logger.WithField("func", "manageFlatPRStrategy").Infof("no changes found, deleting branch %s", targetBranch)
		if err := promoter.client.DeleteBranch(targetBranch); err != nil {
			return "", nil, err
		} else {
			return "no changes found => no pull request necessary", nil, nil
		}
	}
}

func checkForChanges(files []repoaccess.RepositoryFile, files2 []repoaccess.RepositoryFile) bool {
	if len(files) != len(files2) {
		return true
	}
	tempmap := make(map[string]repoaccess.RepositoryFile)
	for _, f := range files {
		tempmap[f.Path] = f
	}
	for _, f2 := range files2 {
		if f, ok := tempmap[f2.Path]; !ok {
			return true
		} else if f.Content != f2.Content {
			return true
		}
	}
	return false
}
