package config

import (
	"gopkg.in/yaml.v2"
)

type PromotionConfigValidator interface {
	Validate(config PromotionConfig) (validationErrors []string)
}

const (
	StrategyBranch string = "branch"
	StrategyFlatPR        = "flat-pr"
)

type PromotionConfig struct {
	APIVersion *string             `yaml:"apiVersion"`
	Kind       *string             `yaml:"kind"`
	Spec       PromotionConfigSpec `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type PromotionConfigSpec struct {
	Strategy *string `yaml:"strategy"`
	Target   Target  `yaml:"target"`
	Paths    []Path  `yaml:"paths"`
}

type Target struct {
	Repo     *string `yaml:"repo"`
	Secret   *string `yaml:"secret"`
	Provider *string `yaml:"provider"`
}

type Path struct {
	Source *string `yaml:"source"`
	Target *string `yaml:"target"`
}

func NewConfig(yamlContent []byte) (*PromotionConfig, error) {

	promotionConfig := PromotionConfig{}
	err := yaml.UnmarshalStrict(yamlContent, &promotionConfig)

	if err != nil {
		return nil, err
	}

	return &promotionConfig, nil
}
