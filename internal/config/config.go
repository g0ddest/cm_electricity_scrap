package config

import (
	"log"
	"os"
)

type Config struct {
	ScrapUri                 string
	ServiceName              string
	DynamoDBLastIdsTableName string
	DynamoDBTableName        string
	SQSQueueURL              string
	AWSRegion                string
	AWSAccessKeyID           string
	AWSSecretAccessKey       string
}

func LoadConfig() Config {

	config := Config{

		ServiceName:              os.Getenv("SERVICE_NAME"),
		ScrapUri:                 os.Getenv("SCRAP_URI"),
		DynamoDBTableName:        os.Getenv("DYNAMODB_TABLE_NAME"),
		SQSQueueURL:              os.Getenv("SQS_QUEUE_URL"),
		AWSRegion:                os.Getenv("AWS_REGION"),
		AWSAccessKeyID:           os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:       os.Getenv("AWS_SECRET_ACCESS_KEY"),
		DynamoDBLastIdsTableName: os.Getenv("DYNAMODB_LAST_IDS_TABLE_NAME"),
	}

	if config.DynamoDBTableName == "" ||
		config.SQSQueueURL == "" ||
		config.AWSRegion == "" ||
		config.AWSAccessKeyID == "" ||
		config.AWSSecretAccessKey == "" ||
		config.ServiceName == "" ||
		config.ScrapUri == "" ||
		config.DynamoDBLastIdsTableName == "" {
		log.Fatalf("One or more environment variables are missing")
	}

	return config
}
