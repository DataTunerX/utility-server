package config

import "github.com/spf13/viper"

var config *viper.Viper

func init() {
	config = viper.New()
	config.AutomaticEnv()
	config.BindEnv("level", "LOG_LEVEL")
	config.SetDefault("level", "debug")
	config.BindEnv("inferenceServiceLabel", "INFERENCE_SERVICE_LABEL")
	config.SetDefault("inferenceServiceLabel", "serviceType=inferenceService")
}

func GetLevel() string {
	return config.GetString("level")
}

func GetInferenceServiceLabel() string {
	return config.GetString("inferenceServiceLabel")
}
