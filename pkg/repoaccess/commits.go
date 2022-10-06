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
	if sourceFileContent, sourceDirContent, resp, err := c.githubInstance.client.Repositories.GetContents(c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	}); err != nil && resp.StatusCode != 404 {
		logger.WithField("func", "GetFilesForBranch").Infof("Response Code: %d", resp.StatusCode)
		return files, err
	} else if resp.StatusCode == 404 {
		return files, nil
	} else if sourceFileContent != nil {
		if content, err := sourceFileContent.GetContent(); err != nil {
			logger.WithField("func", "GetFilesForBranch").Infof("Source File Content: %s", content)
			return files, err
		} else {
			files = append(files, RepositoryFile{
				Content: content,
				Path:    *sourceFileContent.Path,
				SHA:     *sourceFileContent.SHA,
			})
			logger.WithField("func", "GetFilesForBranch").Infof("found file in path %s with content %s with SHA: %s", *sourceFileContent.Path, content, *sourceFileContent.SHA)
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
	return files, nil
}

func (c *Client) SyncFilesWithBranch(branch string, currentTargetFiles, newTargetFiles []RepositoryFile) (changes int, err error) {
	changes = 0

	currentTargetFilesString, err := json.Marshal(currentTargetFiles)
	newTargetFilesString, err := json.Marshal(newTargetFiles)
	logger.WithField("func", "SyncfilesWithBranch").Infof("starting for branch %s and %d currentTargetFiles and %d newTargetFiles", branch, len(currentTargetFiles), len(newTargetFiles))
	logger.WithField("func", "SyncfilesWithBranch").Infof("currentTargetFiles: %s", currentTargetFilesString)
	logger.WithField("func", "SyncfilesWithBranch").Infof("newTargetFiles: %s", newTargetFilesString)

	newTargetFilesMap := make(map[string]RepositoryFile)
	for _, f := range newTargetFiles {
		newTargetFilesMap[f.Path] = f
	}
	currentTargetFilesMap := make(map[string]RepositoryFile)
	for _, f := range currentTargetFiles {
		currentTargetFilesMap[f.Path] = f
	}

	for k, v := range newTargetFilesMap {
		logger.WithField("func", "SyncfilesWithBranch").Infof("New Target File Map. Key: %s, Value: %s", k, v)
		logger.WithField("func", "SyncfilesWithBranch").Infof("Current Target Files Map: %s ", currentTargetFilesMap[k])
		var sourceRepositoryFile *RepositoryFile
		logger.WithField("func", "SyncfilesWithBranch").Infof("#Looking to see if v=k: v: %s k: %s ", v, currentTargetFilesMap[k])
		if v, ok := currentTargetFilesMap[k]; ok {
			sourceRepositoryFile = &v
			logger.WithField("func", "SyncfilesWithBranch").Infof("Found current target file: %s", sourceRepositoryFile)
		} else {
			sourceRepositoryFile = nil
			logger.WithField("func", "SyncfilesWithBranch").Infof("Didnt Find current target file, values where %s and %s", v, currentTargetFilesMap[k])
		}
		if changed, err := c.syncFile(branch, sourceRepositoryFile, k, &v.Content); err != nil {
			return changes, err
		} else if changed {
			changes++
		}
	}
	for k, v := range currentTargetFilesMap {
		if _, ok := newTargetFilesMap[k]; !ok {
			if changed, err := c.syncFile(branch, &v, k, nil); err != nil {
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
			logger.WithField("func", "syncFile").Infof("context: %s, owner %s, repo %s, branch %s content %s", c.githubInstance.context, c.githubInstance.owner, c.githubInstance.repository, branch, *targetFileContent)
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
