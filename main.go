package main

import (
	"context"
	"fmt"
	"keptn/git-promotion-service/pkg/eventhandler"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	logger "github.com/sirupsen/logrus"

	"github.com/keptn/go-utils/pkg/lib/keptn"
	"github.com/keptn/go-utils/pkg/sdk"
)

var keptnOptions = keptn.KeptnOpts{}
var env envConfig

const envVarLogLevel = "LOG_LEVEL"
const serviceName = "git-promotion-service"
const eventWildcard = "*"

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int `envconfig:"RCV_PORT" default:"8080"`
	// RVC Path
	Path string `envconfig:"RCV_PATH" default:"/"`
	// URL of the Keptn API Endpoint
	KeptnAPIURL string `envconfig:"KEPTN_API_URL" required:"true"`
	// The token of the keptn API
	KeptnAPIToken string `envconfig:"KEPTN_API_TOKEN" required:"true"`
}

// NewEventHandler creates a new EventHandler
func NewEventHandler() *eventhandler.EventHandler {
	return &eventhandler.EventHandler{
		ServiceName: serviceName,
		Mapper:      new(eventhandler.KeptnCloudEventMapper),
	}
}

func main() {
	logger.SetLevel(logger.InfoLevel)
	logger.Printf("Starting keptn git promotion service")
	if os.Getenv(envVarLogLevel) != "" {
		logLevel, err := logger.ParseLevel(os.Getenv(envVarLogLevel))
		if err != nil {
			logger.WithError(err).Error("could not parse log level provided by 'LOG_LEVEL' env var")
		} else {
			logger.SetLevel(logLevel)
		}
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var: %s", err)
	}

	_main(os.Args[1:], env)
}

func _main(args []string, env envConfig) {
	keptnOptions.ConfigurationServiceURL = fmt.Sprintf("%s/resource-service", env.KeptnAPIURL)

	log.Printf("Starting %s...", serviceName)
	log.Printf("on Port = %d; Path=%s", env.Port, env.Path)

	ctx := context.Background()
	ctx = cloudevents.WithEncodingStructured(ctx)

	log.Printf("Creating new http handler")

	// Handle all events
	log.Fatal(sdk.NewKeptn(
		serviceName,
		sdk.WithTaskHandler(
			eventWildcard,
			NewEventHandler()),
		sdk.WithLogger(logrus.New()),
	).Start())
}
