package eventhandler

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/keptn/go-utils/pkg/sdk"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/keptn/go-utils/pkg/lib/keptn"

	"keptn/git-promotion-service/pkg/config"
	"keptn/git-promotion-service/pkg/model"
	"keptn/git-promotion-service/pkg/promoter"
	"keptn/git-promotion-service/pkg/repoaccess"

	keptn_interface "keptn/git-promotion-service/pkg/keptn"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
)

const keptnPullRequestTitlePrefix = "keptn:"

type GitPromotionTriggeredEventData struct {
	keptnv2.EventData
}

// EventMapper represents an object able to map the cloudevent (including its data) into a map that will contain the
// parsed JSON of the event data
type EventMapper interface {
	// Map transforms a cloud event into a generic map[string]interface{}
	Map(event sdk.KeptnEvent) (map[string]interface{}, error)
}

// GitPromotionConfigReader retrieves the git promotion configuration
type GitPromotionConfigReader interface {
	GetGitPromotionConfig(gitCommitID string) (*config.PromotionConfig, string, error)
}

// ErrorLogSender is used to send error logs that will appear in Uniform UI
type ErrorLogSender interface {
	SendErrorLogEvent(initialCloudEvent *cloudevents.Event, applicationError error) error
}

// EventHandler contains all information needed to process an event
type EventHandler struct {
	ServiceName     string
	JobConfigReader GitPromotionConfigReader
	Mapper          EventMapper
	ErrorSender     ErrorLogSender
	kubeClient      *kubernetes.Clientset
}

type dataForFinishedEvent struct {
	start time.Time
	end   time.Time
}

// Execute handles all events in a generic manner
func (eh *EventHandler) Execute(k sdk.IKeptn, event sdk.KeptnEvent) (interface{}, *sdk.Error) {
	eventAsInterface, err := eh.Mapper.Map(event)
	if err != nil {
		k.Logger().Errorf("failed to convert incoming cloudevent: %s", err.Error())
		return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: "failed to convert incoming cloudevent: " + err.Error()}
	}

	k.Logger().Infof("Attempting to handle event %s of type %s ...", event.ID, *event.Type)
	k.Logger().Infof("CloudEvent %T: %v", eventAsInterface, eventAsInterface)

	data := &keptnv2.EventData{}
	if err := keptnv2.Decode(event.Data, data); err != nil {
		k.Logger().Errorf("Could not parse event: %s", err.Error())
		return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: fmt.Sprintf("Could not parse event: %s", err.Error())}
	}

	if *event.Type == "sh.keptn.event.git-promotion.triggered" {
		//Get the git commit id from the cloud event (if it exists) and use it to query the job configuration
		var gitCommitID string
		if commitId, ok := eventAsInterface["gitcommitid"]; ok {
			gitCommitID, _ = commitId.(string)
		}

		var gitPromotionConfigReader GitPromotionConfigReader
		if eh.JobConfigReader == nil {
			gitPromotionConfigReader = &config.GitPromotionConfigReader{
				Keptn: keptn_interface.NewV1ResourceHandler(*data, k.APIV2().Resources()),
			}
		} else {
			// Used to pass a mock in the unit tests
			gitPromotionConfigReader = eh.JobConfigReader
		}

		config, configHash, err := gitPromotionConfigReader.GetGitPromotionConfig(gitCommitID)
		if err != nil {
			k.Logger().Infof("Cant get git promotion config %s", gitCommitID)
			return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: "could not retrieve config for git-promotion-service: " + err.Error()}
		}

		yamlString, err := yaml.Marshal(&config)
		k.Logger().Infof("Project: %s Config Hash: %s", data.GetProject(), configHash)
		k.Logger().Infof("Configuration: %s", yamlString)

		nextStage := getNextStage(data.GetStage())
		k.Logger().Infof("Next Stage %s", nextStage)

		placeholders := map[string]string{
			"project":   data.GetProject(),
			"stage":     data.GetStage(),
			"nextstage": nextStage,
			"service":   data.GetService(),
		}

		config.Spec.Target.Repo = replacePlaceHolders(placeholders, config.Spec.Target.Repo)
		config.Spec.Target.Secret = replacePlaceHolders(placeholders, config.Spec.Target.Secret)
		for i, p := range config.Spec.Paths {
			p.Target = replacePlaceHolders(placeholders, p.Target)
			p.Source = replacePlaceHolders(placeholders, p.Source)
			config.Spec.Paths[i] = p
		}

		// accessToken, err := eh.getAccessToken(*config.Spec.Target.Secret)
		// if err != nil {
		// 	k.Logger().Errorf("handleGitPromotionTriggeredEvent: error while reading secret with name %s", *config.Spec.Target.Secret)
		// 	//sendTaskFailedEvent(&keptnv2.Keptn{}, "git-promotion", "git-promotion-service", err, "Could get Access Token")
		// }

		accessToken := os.Getenv("GIT_ACCESS_TOKEN")
		client, err := repoaccess.NewClient(accessToken, *config.Spec.Target.Repo)
		if err != nil {
			k.Logger().Errorf("handleGitPromotionTriggeredEvent: error while creating client for repo: %s", err.Error())
		}

		res := make(map[string]string)
		addKeysToMap("data", &res, eventAsInterface)
		//addKeysToMap("", &res, event.Extensions)
		res["id"] = event.ID
		res["source"] = *event.Source
		res["specversion"] = event.Specversion

		var paths []model.Path
		for _, path := range config.Spec.Paths {
			var x model.Path
			x.Source = path.Source
			x.Target = path.Target
			paths = append(paths, x)
		}

		yamlPath, err := yaml.Marshal(paths)
		k.Logger().Infof("*$* paths %s *$*", yamlPath)

		p := promoter.NewFlatPrPromoter(client)
		if msg, prlink, err := p.Promote(*config.Spec.Target.Repo, res, "main",
			buildBranchName(data.GetStage(), nextStage, event.Shkeptncontext),
			buildTitle(event.Shkeptncontext, nextStage),
			buildBody(event.Shkeptncontext, data.GetProject(), data.GetService(), data.GetStage()), paths); err != nil {
			log.Printf("flat pr strategy failed on repository %s Message %s", *config.Spec.Target.Repo, msg)
			k.Logger().Errorf("Error while opening PR: %s", err.Error())
		} else {
			log.Printf("PR Done %s", *prlink)
		}

		k.Logger().Infof("Getting task finished event")
		return getTaskFinishedEvent(event, data), nil
	}

	return getTaskFinishedEvent(event, data), nil

}

func sendTaskFailedEvent(myKeptn *keptnv2.Keptn, taskName string, serviceName string, err error, logs string) {
	var message string

	if logs != "" {
		message = fmt.Sprintf("Task '%s' failed: %s\n\nLogs: \n%s", taskName, err.Error(), logs)
	} else {
		message = fmt.Sprintf("Task '%s' failed: %s", taskName, err.Error())
	}

	_, err = myKeptn.SendTaskFinishedEvent(
		&keptnv2.EventData{
			Status:  keptnv2.StatusErrored,
			Result:  keptnv2.ResultFailed,
			Message: message,
		}, serviceName,
	)

	if err != nil {
		log.Printf("Error while sending started event: %s\n", err)
	}
}

func sendJobFailedEvent(myKeptn *keptnv2.Keptn, jobName string, serviceName string, err error) {
	_, err = myKeptn.SendTaskFinishedEvent(
		&keptnv2.EventData{
			Status:  keptnv2.StatusErrored,
			Result:  keptnv2.ResultFailed,
			Message: fmt.Sprintf("Job %s failed: %s", jobName, err.Error()),
		}, serviceName,
	)

	if err != nil {
		log.Printf("Error while sending started event: %s\n", err)
	}
}

// getTaskFinishedEvent returns the finished data for the received event as an interface which can be directly returned using the go-sdk
func getTaskFinishedEvent(event sdk.KeptnEvent, receivedEventData keptn.EventProperties) interface{} {
	var logMessage strings.Builder

	eventData := &keptnv2.EventData{
		Status:  keptnv2.StatusSucceeded,
		Result:  keptnv2.ResultPass,
		Message: logMessage.String(),
		Project: receivedEventData.GetProject(),
		Stage:   receivedEventData.GetStage(),
		Service: receivedEventData.GetService(),
	}

	return eventData
}

func isTestTriggeredEvent(eventName string) bool {
	return eventName == keptnv2.GetTriggeredEventType(keptnv2.TestTaskName)
}

func replacePlaceHolders(placeholders map[string]string, p *string) (result *string) {
	if p == nil {
		return nil
	}
	current := *p
	for k, v := range placeholders {
		current = strings.Replace(current, fmt.Sprintf("${%s}", k), v, -1)
	}
	return &current
}

// func readAndMergeResource(target model.PromotionConfig, getResourceFunc func() (resource *models.Resource, err error)) (ret model.PromotionConfig) {
// 	ret = target
// 	resource, err := getResourceFunc()
// 	if err == api.ResourceNotFoundError {
// 		return ret
// 	}
// 	var newConfig model.PromotionConfig
// 	if err := yaml.Unmarshal([]byte(resource.ResourceContent), &newConfig); err != nil {
// 		logger.WithField("func", "readAndMergeResource").
// 			WithError(err).
// 			Errorf("could not unmarshall resource file %s => ignoring", *resource.ResourceURI)
// 	} else {
// 		if newConfig.Spec.Strategy != nil {
// 			ret.Spec.Strategy = newConfig.Spec.Strategy
// 		}
// 		if newConfig.Spec.Target.Repo != nil {
// 			ret.Spec.Target.Repo = newConfig.Spec.Target.Repo
// 		}
// 		if newConfig.Spec.Target.Secret != nil {
// 			ret.Spec.Target.Secret = newConfig.Spec.Target.Secret
// 		}
// 		if newConfig.Spec.Target.Provider != nil {
// 			ret.Spec.Target.Provider = newConfig.Spec.Target.Provider
// 		}
// 		ret.Spec.Paths = append(target.Spec.Paths, newConfig.Spec.Paths...)
// 	}
// 	return ret
// }

func (eh *EventHandler) getAccessToken(secretName string) (accessToken string, err error) {
	if secret, err := eh.kubeClient.CoreV1().Secrets(os.Getenv("K8S_NAMESPACE")).Get(context.Background(), secretName, v1.GetOptions{}); err != nil {
		return accessToken, err
	} else {
		log.Printf("found access-token with length %d in secret %s", len(secret.Data["access-token"]), secret.Name)
		return string(secret.Data["access-token"]), nil
	}
}

func buildBranchName(stage string, nextStage string, shkeptncontext string) string {
	return fmt.Sprintf("promote/%s_%s-%s", stage, nextStage, shkeptncontext)
}

func buildTitle(keptncontext, nextStage string) string {
	return fmt.Sprintf("%s Promote to stage %s (ctx: %s)", keptnPullRequestTitlePrefix, nextStage, keptncontext)
}

func buildBody(keptncontext, projectName, serviceName, stage string) string {
	return fmt.Sprintf(`Opened by keptn sequence [%s](%s/bridge/project/%s/sequence/%s/stage/%s).

Project: *%s* 
Service: *%s* 
Stage: *%s*`, keptncontext, os.Getenv("EXTERNAL_URL"), projectName, keptncontext, stage, projectName, serviceName, stage)
}

func getNextStage(stage string) (nextStage string) {
	if stage == "test" {
		return "production"
	}
	return "production"
}

func addKeysToMap(root string, m *map[string]string, temp map[string]interface{}) {
	for k, v := range temp {
		key := k
		if root != "" {
			key = root + "." + k
		}
		if v != nil {
			if reflect.TypeOf(v).Kind() != reflect.Map {
				(*m)[key] = fmt.Sprintf("%v", v)
			} else {
				addKeysToMap(key, m, v.(map[string]interface{}))
			}
		}
	}
}
