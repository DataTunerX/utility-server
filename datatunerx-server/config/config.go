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
	config.BindEnv("s3ServiceEndpoint", "S3_SERVICE_ENDPOINT")
	config.BindEnv("s3ServiceAccessKey", "S3_SERVICE_ACCESSKEY")
	config.BindEnv("s3ServiceSecretKey", "S3_SERVICE_SECRETKEY")
	config.BindEnv("s3ServiceUseSSL", "S3_SERVICE_USESSL")
	config.SetDefault("s3ServiceUseSSL", false)
}

func GetLevel() string {
	return config.GetString("level")
}

func GetInferenceServiceLabel() string {
	return config.GetString("inferenceServiceLabel")
}

func GetS3ServiceEndpoint() string {
	return config.GetString("s3ServiceEndpoint")
}

func GetS3ServiceAccessKey() string {
	return config.GetString("s3ServiceAccessKey")
}

func GetS3ServiceSecretKey() string {
	return config.GetString("s3ServiceSecretKey")
}

func GetS3ServiceUseSSL() bool {
	return config.GetBool("s3ServiceUseSSL")
}
