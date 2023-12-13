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
	config.BindEnv("rayServiceGroup", "RAYGROUP")
	config.SetDefault("rayServiceGroup", "ray.io")
	config.BindEnv("rayServiceVersion", "RAY_SERVICE_VERSION")
	config.SetDefault("rayServiceVersion", "v1")
	config.BindEnv("rayServiceResource", "RAY_SERVICE_RESOURCE")
	config.SetDefault("rayServiceResource", "rayservices")
}

func GetLevel() string {
	return config.GetString("level")
}

func GetInferenceServiceLabel() string {
	return config.GetString("inferenceServiceLabel")
}

func GetRayServiceGroup() string {
	return config.GetString("rayServiceGroup")
}

func GetRayServiceVersion() string {
	return config.GetString("rayServiceVersion")
}

func GetRayServiceResource() string {
	return config.GetString("rayServiceResource")
}
