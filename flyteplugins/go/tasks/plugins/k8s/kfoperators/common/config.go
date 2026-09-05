package common

import (
	"time"

	pluginsConfig "github.com/flyteorg/flyte/flyteplugins/go/tasks/config"
	"github.com/flyteorg/flyte/flytestdlib/config"
)

//go:generate pflags Config --default-var=defaultConfig

var (
	defaultConfig = Config{
		Timeout:                        config.Duration{Duration: 1 * time.Minute},
		DefaultTTLSecondsAfterFinished: -1,
	}

	configSection = pluginsConfig.MustRegisterSubSection("kf-operator", &defaultConfig)
)

// Config is config for kubeflow operator plugins (pytorch, tensorflow).
type Config struct {
	// If kubeflow operator doesn't update the status of the task after this timeout, the task will be considered failed.
	Timeout config.Duration `json:"timeout,omitempty"`

	// DefaultCleanPodPolicy sets the default CleanPodPolicy on training jobs when
	// the task spec does not provide one. Valid values: "None", "Running", "All".
	// When empty, no default is applied and the training operator's own default is used.
	DefaultCleanPodPolicy string `json:"defaultCleanPodPolicy,omitempty"`

	// DefaultTTLSecondsAfterFinished sets a default TTL on finished training jobs
	// when the task spec does not provide one. The training operator will garbage-
	// collect the job and its pods after this many seconds. 0 means immediate cleanup.
	// -1 (default) means no default is applied.
	DefaultTTLSecondsAfterFinished int32 `json:"defaultTTLSecondsAfterFinished,omitempty"`
}

func GetConfig() *Config {
	return configSection.GetConfig().(*Config)
}

func SetConfig(cfg *Config) error {
	return configSection.SetConfig(cfg)
}
