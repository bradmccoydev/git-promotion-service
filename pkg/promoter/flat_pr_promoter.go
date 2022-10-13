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
	//"starting flat pr strategy with sourceBranch main and targetBranch promote/integration-test_production-b0501292-d94d-4149-a98d-406aca2b9473 and fields map[data.data.message: data.data.project:ortelius data.data.result:pass data.data.service:podtatohead data.data.stage:integration-test data.data.status:succeeded data.data.temporaryData.distributor.subscriptionID:71b1a157-2da1-4a97-91d7-8759f4997473 data.gitcommitid:173c84a53f6d0ce87d455de6fff3b15364eb5485 data.id:3c784266-4146-4387-819d-f5a3be6292b6 data.shkeptncontext:b0501292-d94d-4149-a98d-406aca2b9473 data.source:0xc00019e4a0 data.specversion:1.0 data.time:2022-10-11 07:07:35.695714903 +0000 UTC data.type:0xc00019e4b0 id:3c784266-4146-4387-819d-f5a3be6292b6 source:shipyard-controller specversion:1.0]"
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
	//"processing 1 paths"
	for _, p := range paths {
		var path string
		if p.Source == nil {
			path = *p.Target
		} else {
			path = *p.Source
		}

		logger.WithField("func", "manageFlatPRStrategy").Infof("Getting Files for branch %s path %s", sourceBranch, path)
		//"Getting Files for branch main path /kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml"
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
		//"pCurrentTargetFilesJSON [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.5\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml\",\"SHA\":\"5d35811e14d7aa9f9eccd9739ae5e89683a08cd7\"}]"
		logger.WithField("func", "manageFlatPRStrategy").Infof("pNewTargetFiles %s", pNewTargetFilesJSON)
		//"pNewTargetFiles [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.7\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml\",\"SHA\":\"6f244d800c062740187b1893b2d41551c94ae02a\"}]"

		for i, c := range pNewTargetFiles {
			pNewTargetFiles[i].Content = replacer.Replace(c.Content, fields)
			if p.Source != nil {
				pNewTargetFiles[i].Path = strings.Replace(pNewTargetFiles[i].Path, *p.Source, *p.Target, -1)
			}
		}

		changedpNewTargetFilesJSON, err := json.Marshal(pNewTargetFiles)
		logger.WithField("func", "manageFlatPRStrategy").Infof("Modified %s", changedpNewTargetFilesJSON)
		//"Modified [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.7\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml\",\"SHA\":\"6f244d800c062740187b1893b2d41551c94ae02a\"}]"

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
