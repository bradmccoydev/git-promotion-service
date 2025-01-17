package main

import (
	"context"
	"fmt"
	"keptn/git-promotion-service/pkg/handler"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	logger "github.com/sirupsen/logrus"

	keptnapi "github.com/keptn/go-utils/pkg/api/utils"
	api "github.com/keptn/go-utils/pkg/api/utils/v2"
	"github.com/keptn/go-utils/pkg/lib/keptn"
	keptncommon "github.com/keptn/go-utils/pkg/lib/keptn"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
)

var keptnOptions = keptn.KeptnOpts{}
var env envConfig

const envVarLogLevel = "LOG_LEVEL"
const ServiceName = "git-promotion-service"

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

// Opaque key type used for graceful shutdown context value
type gracefulShutdownKeyType struct{}

var gracefulShutdownKey = gracefulShutdownKeyType{}

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

	os.Exit(_main(os.Args[1:], env))
}

func _main(args []string, env envConfig) int {
	ctx := getGracefulContext()

	keptnOptions.ConfigurationServiceURL = fmt.Sprintf("%s/resource-service", env.KeptnAPIURL)

	p, err := cloudevents.NewHTTP(cloudevents.WithPath(env.Path), cloudevents.WithPort(env.Port), cloudevents.WithGetHandlerFunc(keptnapi.HealthEndpointHandler))
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	c, err := cloudevents.NewClient(p)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	log.Fatal(c.StartReceiver(ctx, gotEvent))

	return 0
}

func gotEvent(ctx context.Context, event cloudevents.Event) error {
	ctx.Value(gracefulShutdownKey).(*sync.WaitGroup).Add(1)
	val := ctx.Value(gracefulShutdownKey)
	if val != nil {
		if wg, ok := val.(*sync.WaitGroup); ok {
			wg.Add(1)
		}
	}
	go switchEvent(ctx, event)
	return nil
}

func switchEvent(ctx context.Context, event cloudevents.Event) {
	defer func() {
		val := ctx.Value(gracefulShutdownKey)
		if val == nil {
			return
		}
		if wg, ok := val.(*sync.WaitGroup); ok {
			wg.Done()
		}
	}()
	keptnHandlerV2, err := keptnv2.NewKeptn(&event, keptncommon.KeptnOpts{})
	if err != nil {
		logger.WithError(err).Error("failed to initialize Keptn handler")
		return
	}

	apiSet, err := api.New(os.Getenv("API_BASE_URL"), api.WithAuthToken(os.Getenv("API_AUTH_TOKEN")))
	if err != nil {
		logger.WithError(err).Error("failed to initialize API Set")
		return
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		logger.WithError(err).Error("failed to initialize kube client rest")
		return
	}
	kubeAPI, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.WithError(err).Error("failed to initialize kube client config")
		return
	}

	handlers := []handler.Handler{
		handler.NewGitPromotionTriggeredEventHandler(keptnHandlerV2, apiSet, kubeAPI),
	}

	unhandled := true
	for _, currHandler := range handlers {
		if currHandler.IsTypeHandled(event) {
			unhandled = false
			currHandler.Handle(event, keptnHandlerV2)
		}
	}

	if unhandled {
		logger.Debugf("Received unexpected keptn event type %s", event.Type())
	}
}

func getGracefulContext() context.Context {

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), gracefulShutdownKey, wg))
	ctx = cloudevents.WithEncodingStructured(ctx)
	go func() {
		<-ch
		logger.Fatal("Container termination triggered, starting graceful shutdown")
		wg.Wait()
		logger.Fatal("cancelling context")
		cancel()
	}()
	return ctx
}
