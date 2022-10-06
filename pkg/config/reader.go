package config

import (
	"fmt"
	"log"

	"golang.org/x/crypto/sha3"
)

// Needs to be escaped manually when sending it to the api-gateway-nginx
const gitPromotionConfigResourceName = "git-promotion.yaml"

//go:generate mockgen -destination=fake/reader_mock.go -package=fake . KeptnResourceService

// KeptnResourceService defines the contract used by gitPromotionConfigReader to retrieve a resource from Keptn
// The project, service, stage environment variables are taken from the context of the ResourceService (Event)
type KeptnResourceService interface {
	// GetServiceResource returns the service level resource
	GetServiceResource(resource string, gitCommitID string) ([]byte, error)

	// GetProjectResource returns the resource that was defined on project level
	GetProjectResource(resource string, gitCommitID string) ([]byte, error)

	// GetStageResource returns the resource that was defined in the stage
	GetStageResource(resource string, gitCommitID string) ([]byte, error)

	// GetAllKeptnResources returns all resources that were defined in the stage
	GetAllKeptnResources(resource string) (map[string][]byte, error)
}

// GitPromotionConfigReader retrieves and parses git promotion configuration from Keptn
type GitPromotionConfigReader struct {
	Keptn KeptnResourceService
}

// FindGitPromotionConfigResource searches for the job configuration resource in the service, stage and then the project
// and returns the content of the first resource that is found
func (jcr *GitPromotionConfigReader) FindGitPromotionConfigResource(gitCommitID string) ([]byte, error) {
	if config, err := jcr.Keptn.GetServiceResource(gitPromotionConfigResourceName, gitCommitID); err == nil {
		return config, nil
	}

	if config, err := jcr.Keptn.GetStageResource(gitPromotionConfigResourceName, gitCommitID); err == nil {
		return config, nil
	}

	// NOTE: Since the resource service uses different branches, the commitID may not be in the main
	//       branch and therefore it's not possible to query the project fallback configuration!
	if config, err := jcr.Keptn.GetProjectResource(gitPromotionConfigResourceName, ""); err == nil {
		return config, nil
	}

	return nil, fmt.Errorf("unable to find git promotion configuration")
}

// Getgit promotionConfig retrieves job/config.yaml resource from keptn and parses it into a Config struct.
// Additionally, also the SHA1 hash of the retrieved configuration will be returned.
// In case of error retrieving the resource or parsing the yaml it will return (nil,
// error) with the original error correctly wrapped in the local one
func (jcr *GitPromotionConfigReader) GetGitPromotionConfig(gitCommitID string) (*PromotionConfig, string, error) {

	resource, err := jcr.FindGitPromotionConfigResource(gitCommitID)
	if err != nil {
		return nil, "", fmt.Errorf("error retrieving git promotion config: %w", err)
	}

	hasher := sha3.New224()
	hasher.Write(resource)
	resourceHashBytes := hasher.Sum(nil)
	resourceHash := fmt.Sprintf("%x", resourceHashBytes)

	configuration, err := NewConfig(resource)
	if err != nil {
		log.Printf("Could not parse config: %s", err)
		log.Printf("The config was: %s", string(resource))
		return nil, "", fmt.Errorf("error parsing job configuration: %w", err)
	}

	return configuration, resourceHash, nil
}
