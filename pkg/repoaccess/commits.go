package repoaccess

import (
	"encoding/json"

	"github.com/google/go-github/github"
	logger "github.com/sirupsen/logrus"
)

func (c *Client) CheckForNewCommits(toBranch, fromBranch string) (newCommits bool, err error) {
	compare, _, err := c.githubInstance.client.Repositories.CompareCommits(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository, toBranch, fromBranch)
	if err != nil {
		return false, err
	}
	logger.WithField("func", "CheckForNewCommits").Infof("found %d commits in github repo %s/%s from branch %s to %s", len(compare.Commits), c.githubInstance.owner, c.githubInstance.repository, fromBranch, toBranch)
	if len(compare.Commits) == 0 {
		return false, nil
	} else {
		return true, nil
	}
}

type RepositoryFile struct {
	Content string
	Path    string
	SHA     string
}

func (c *Client) GetFilesForBranch(branch, path string) (files []RepositoryFile, err error) {
	logger.WithField("func", "GetFilesForBranch").Infof("starting with branch %s and path %s", branch, path)
	//"starting with branch main and path /kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml"
	//"starting with branch main and path /kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml"
	sourceFileContent, sourceDirContent, resp, err := c.githubInstance.client.Repositories.GetContents(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository, path, &github.RepositoryContentGetOptions{Ref: branch})
	logger.WithField("func", "GetFilesForBranch").Infof("The Response Code: %d", resp.StatusCode)
	//"The Response Code: 200"
	//"The Response Code: 200"

	if err != nil && resp.StatusCode != 404 {
		logger.WithField("func", "GetFilesForBranch").Infof("Response Code: %d", resp.StatusCode)
		return files, err
	} else if resp.StatusCode == 404 {
		logger.WithField("func", "GetFilesForBranch").Info("404 when Getting contents")
		logger.WithField("func", "GetFilesForBranch").Infof("GetContents Resp: %s", resp)
		return files, nil
	} else if sourceFileContent != nil {
		logger.WithField("func", "GetFilesForBranch").Info("Source File Content is not nil")
		//Source File Content is not nil
		//Source File Content is not nil
		if content, err := sourceFileContent.GetContent(); err != nil {
			logger.WithField("func", "GetFilesForBranch").Infof("The content is: %s and error is %s ", content, err.Error())
			return files, err
		} else {
			logger.WithField("func", "GetFilesForBranch").Info("Appending Files to repository file.")
			//Appending Files to repository file
			//Appending Files to repository file
			files = append(files, RepositoryFile{
				Content: content,
				Path:    *sourceFileContent.Path,
				SHA:     *sourceFileContent.SHA,
			})
			logger.WithField("func", "GetFilesForBranch").Infof("found file in path %s with content %s with SHA: %s", *sourceFileContent.Path, content, *sourceFileContent.SHA)
			//"found file in path kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml with content apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n with SHA: 6f244d800c062740187b1893b2d41551c94ae02a"
			//"found file in path kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml with content apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.5\n with SHA: 5d35811e14d7aa9f9eccd9739ae5e89683a08cd7"
		}
	} else {
		for _, sf := range sourceDirContent {
			logger.WithField("func", "GetFilesForBranch").Infof("processing entry with path %s", *sf.Path)
			if *sf.Type == "file" {
				if contentsf, _, _, err := c.githubInstance.client.Repositories.GetContents(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository, *sf.Path, &github.RepositoryContentGetOptions{}); err != nil {
					return files, err
				} else {
					if content, err := contentsf.GetContent(); err != nil {
						return files, err
					} else {
						files = append(files, RepositoryFile{
							Content: content,
							Path:    *sf.Path,
							SHA:     *sf.SHA,
						})
						logger.WithField("func", "GetFilesForBranch").Infof("found file in path %s with content %s", *sf.Path, content)
					}
				}
			} else if *sf.Type == "dir" {
				if dirFiles, err := c.GetFilesForBranch(branch, *sf.Path); err != nil {
					return files, err
				} else {
					files = append(files, dirFiles...)
				}
			} else {
				logger.WithField("func", "GetFilesForBranch").Infof("unknown file type %s", *sf.Type)
			}
		}
	}
	finalFiles, err := json.Marshal(files)
	logger.WithField("func", "GetFilesForBranch").Infof("Final Files: %s", finalFiles)
	//"Final Files: [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.7\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml\",\"SHA\":\"6f244d800c062740187b1893b2d41551c94ae02a\"}]"
	//"Final Files: [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.5\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml\",\"SHA\":\"5d35811e14d7aa9f9eccd9739ae5e89683a08cd7\"}]"
	return files, nil
}

func (c *Client) SyncFilesWithBranch(branch string, currentTargetFiles, newTargetFiles []RepositoryFile) (changes int, err error) {
	changes = 0

	currentTargetFilesString, err := json.Marshal(currentTargetFiles)
	newTargetFilesString, err := json.Marshal(newTargetFiles)
	logger.WithField("func", "SyncfilesWithBranch").Infof("starting for branch %s and %d currentTargetFiles and %d newTargetFiles", branch, len(currentTargetFiles), len(newTargetFiles))
	//"starting for branch promote/integration-test_production-b0501292-d94d-4149-a98d-406aca2b9473 and 1 currentTargetFiles and 1 newTargetFiles"
	logger.WithField("func", "SyncfilesWithBranch").Infof("currentTargetFiles: %s", currentTargetFilesString)
	//"currentTargetFiles: [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.5\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml\",\"SHA\":\"5d35811e14d7aa9f9eccd9739ae5e89683a08cd7\"}]"
	logger.WithField("func", "SyncfilesWithBranch").Infof("newTargetFiles: %s", newTargetFilesString)
	//"newTargetFiles: [{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.7\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml\",\"SHA\":\"6f244d800c062740187b1893b2d41551c94ae02a\"}]"

	newTargetFilesMap := make(map[string]RepositoryFile)
	for _, f := range newTargetFiles {
		newTargetFilesMap[f.Path] = f
	}
	currentTargetFilesMap := make(map[string]RepositoryFile)
	for _, f := range currentTargetFiles {
		currentTargetFilesMap[f.Path] = f
	}

	updatedNewTargetFilesMap, err := json.Marshal(newTargetFilesMap)
	logger.WithField("func", "SyncfilesWithBranch").Infof("updatedNewTargetFilesMap: %s ", updatedNewTargetFilesMap)
	//"updatedNewTargetFilesMap: {\"kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml\":{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.7\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml\",\"SHA\":\"6f244d800c062740187b1893b2d41551c94ae02a\"}} "
	updatedCurrentTargetFilesMap, err := json.Marshal(currentTargetFilesMap)
	logger.WithField("func", "SyncfilesWithBranch").Infof("updatedCurrentTargetFilesMap: %s ", updatedCurrentTargetFilesMap)
	//"updatedCurrentTargetFilesMap: {\"kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml\":{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.5\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml\",\"SHA\":\"5d35811e14d7aa9f9eccd9739ae5e89683a08cd7\"}} "
	for k, v := range newTargetFilesMap {
		logger.WithField("func", "SyncfilesWithBranch").Infof("New Target File Map. Key: %s, Value: %s", k, v)
		//"New Target File Map. Key: kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml, Value: {apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml 6f244d800c062740187b1893b2d41551c94ae02a}"
		logger.WithField("func", "SyncfilesWithBranch").Infof("Current Target Files Map: %s ", currentTargetFilesMap)
		//"Current Target Files Map: map[kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml:{apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.5\n kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml 5d35811e14d7aa9f9eccd9739ae5e89683a08cd7}] "
		logger.WithField("func", "SyncfilesWithBranch").Infof("New Target Files Map: %s ", newTargetFilesMap)
		//"New Target Files Map: map[kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml:{apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml 6f244d800c062740187b1893b2d41551c94ae02a}] "

		var sourceRepositoryFile *RepositoryFile
		// var sourceRepositoryFile1 *RepositoryFile
		// sourceRepositoryFile1.SHA = "5d35811e14d7aa9f9eccd9739ae5e89683a08cd7"
		// sourceRepositoryFile1.Path = "kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml"
		// sourceRepositoryFile1.Content = "[{\"Content\":\"apiVersion: apps/v1\\nkind: Deployment\\nmetadata:\\n  name: podtato-head-left-arm\\nspec:\\n  template:\\n    spec:\\n      containers:\\n      - name: podtato-head-left-arm\\n        image: ghcr.io/podtato-head/left-arm:0.2.5\\n\",\"Path\":\"kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml\",\"SHA\":\"5d35811e14d7aa9f9eccd9739ae5e89683a08cd7\"}]"

		logger.WithField("func", "SyncfilesWithBranch").Infof("#Looking to see if v=k: v: %s k: %s ", v, currentTargetFilesMap[k])
		//"#Looking to see if v=k: v: {apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml 6f244d800c062740187b1893b2d41551c94ae02a} k: {  } "
		if v, ok := currentTargetFilesMap[k]; ok {
			//LOG: v: {apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml 6f244d800c062740187b1893b2d41551c94ae02a}
			//LOG: k: {  }
			sourceRepositoryFile = &v
			logger.WithField("func", "SyncfilesWithBranch").Infof("Found current target file: %s", sourceRepositoryFile)
		} else {
			sourceRepositoryFile = nil
			logger.WithField("func", "SyncfilesWithBranch").Infof("Didnt Find current target file, values where %s and %s", v, currentTargetFilesMap[k])
			//LOG: "Didnt Find current target file, values where {  } and {  }"
		}

		// test, err := c.syncFile(branch, sourceRepositoryFile, "kube-infra/kustomize/podtato-head/podtato-head/envs/qa/version.yaml", &v.Content)
		// if err != nil {
		// 	logger.WithField("func", "SyncfilesWithBranch").Infof("My Test %t error: %s", test, err.Error())
		// }

		//logger.WithField("func", "SyncfilesWithBranch").Infof("Did it work %t ", test)

		if changed, err := c.syncFile(branch, sourceRepositoryFile, k, &v.Content); err != nil {
			if changed, err := c.syncFile(branch, sourceRepositoryFile, k, &v.Content); err != nil { //target path should be prod
				logger.WithField("func", "SyncfilesWithBranch").Infof("Couldnt SyncFile: %s", err.Error())
				logger.WithField("func", "SyncfilesWithBranch").Infof("Its Changed: %t", changed)
			}
			//"Couldnt SyncFile: PUT https://api.github.com/repos/ortelius/ortelius-kubernetes/contents/kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml: 422 Invalid request.\n\n\"sha\" wasn't supplied. []"
			return changes, err
		} else if changed {
			changes++
		}
	}
	for k, v := range currentTargetFilesMap {
		if _, ok := newTargetFilesMap[k]; !ok {
			if changed, err := c.syncFile(branch, &v, k, nil); err != nil { //target path should be prod
				logger.WithField("func", "SyncfilesWithBranch").Infof("Didnt Sync Files: %s", err.Error())
				return changes, err
			} else if changed {
				changes++
			}
		}
	}
	return changes, nil
}

func (c *Client) syncFile(branch string, currentFile *RepositoryFile, targetPath string, targetFileContent *string) (changed bool, err error) {
	logger.WithField("func", "syncFile").Infof("starting with branch %s, targetPath %s", branch, targetPath)
	//"starting with branch promote/integration-test_production-b0501292-d94d-4149-a98d-406aca2b9473, targetPath kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml"
	logger.WithField("func", "syncFile").Infof("Target File Content: %s", *targetFileContent)
	//"Target File Content: apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n"
	if currentFile == nil && targetFileContent == nil {
		logger.WithField("func", "syncFile").Infof("both contents are nil for branch %s and targetPath %s => doing nothing", branch, targetPath)
		return false, nil
	}
	author := &github.CommitAuthor{
		Name:  github.String("keptn"),
		Email: github.String("keptn-no-reply@github.com"),
	}

	if targetFileContent == nil {
		logger.WithField("func", "syncFile").Infof("deleting file %s in branch %s", currentFile.Path, branch)
		if _, _, err := c.githubInstance.client.Repositories.DeleteFile(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository,
			currentFile.Path, &github.RepositoryContentFileOptions{
				Message:   github.String("(build) delete file"),
				Branch:    github.String(branch),
				Author:    author,
				Committer: author,
				SHA:       github.String(currentFile.SHA),
			}); err != nil {
			return false, err
		} else {
			changed = true
		}
	} else {
		if currentFile == nil {
			logger.WithField("func", "syncFile").Infof("creating file %s in branch %s", targetPath, branch)
			//"creating file kube-infra/kustomize/podtato-head/podtato-head/envs/int/version.yaml in branch promote/integration-test_production-b0501292-d94d-4149-a98d-406aca2b9473"
			logger.WithField("func", "syncFile").Infof("context: %s, owner %s, repo %s, branch %s content %s", c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository, branch, *targetFileContent)
			//"context: context.Background, owner ortelius, repo ortelius-kubernetes, branch promote/integration-test_production-b0501292-d94d-4149-a98d-406aca2b9473 content apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n"
			if _, _, err := c.githubInstance.client.Repositories.CreateFile(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository,
				targetPath, &github.RepositoryContentFileOptions{
					Message:   github.String("(build) create file"),
					Branch:    github.String(branch),
					Author:    author,
					Committer: author,
					Content:   []byte(*targetFileContent),
				}); err != nil {
				return false, err
			} else {
				changed = true
			}
		} else {
			if currentFile.Content != *targetFileContent {
				logger.WithField("func", "syncFile").Infof("updating file %s in branch %s", targetPath, branch)
				if _, _, err := c.githubInstance.client.Repositories.UpdateFile(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository,
					targetPath, &github.RepositoryContentFileOptions{
						Message:   github.String("(build) update file"),
						Branch:    github.String(branch),
						SHA:       github.String(currentFile.SHA),
						Author:    author,
						Committer: author,
						Content:   []byte(*targetFileContent),
					}); err != nil {
					return changed, err
				} else {
					changed = true
				}
			} else {
				logger.WithField("func", "syncFile").Infof("ignoring file %s in branch %s (no changes detected)", targetPath, branch)
			}
		}
	}
	return changed, nil
}
