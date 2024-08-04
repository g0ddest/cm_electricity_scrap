package handlers

import (
	"cm_electricity_scrap/internal/config"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
)

type Message struct {
	ID         string `json:"id"`
	Service    string `json:"service"`
	CreatedAt  string `json:"created_at"`
	RawMessage string `json:"raw_message"`
	Source     Source `json:"source"`
}

type Source struct {
	Channel    string `json:"channel"`
	SourceURI  string `json:"source_uri"`
	SenderName string `json:"sender_name"`
	SenderURI  string `json:"sender_uri"`
}

func ScrapAndProcess(cfg config.Config) error {
	lastID, err := getLastID(cfg)
	if err != nil {
		return fmt.Errorf("failed to get last ID: %w", err)
	}

	currentID, err := strconv.Atoi(lastID)
	if err != nil {
		return fmt.Errorf("failed to convert last ID to int: %w", err)
	}

	for {
		currentID++
		url := fmt.Sprintf(cfg.ScrapUri, currentID)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to fetch URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("URL %s returned %d, skipping", url, resp.StatusCode)
			break
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to parse HTML: %w", err)
		}

		content, err := extractContent(doc)
		if err != nil {
			return fmt.Errorf("failed to extract content: %w", err)
		}

		id := uuid.New().String()
		now := time.Now().Format(time.RFC3339)

		message := Message{
			ID:         id,
			Service:    cfg.ServiceName,
			CreatedAt:  now,
			RawMessage: content,
			Source: Source{
				Channel:    "web",
				SourceURI:  url,
				SenderName: "",
				SenderURI:  "",
			},
		}

		if err := saveToDynamoDB(cfg, message); err != nil {
			return fmt.Errorf("failed to save to DynamoDB: %w", err)
		}

		if err := sendMessageToSQS(cfg, message); err != nil {
			return fmt.Errorf("failed to send message to SQS: %w", err)
		}

		if err := updateLastID(cfg, strconv.Itoa(currentID)); err != nil {
			return fmt.Errorf("failed to update last ID: %w", err)
		}
	}

	return nil
}

func getLastID(cfg config.Config) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(cfg.AWSRegion),
		Credentials: credentials.NewStaticCredentials(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
	}))
	svc := dynamodb.New(sess)

	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(cfg.DynamoDBLastIdsTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(cfg.ServiceName)},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get item from DynamoDB: %w", err)
	}

	if result.Item == nil {
		return "", fmt.Errorf("no item found in DynamoDB")
	}

	return *result.Item["last_id"].S, nil
}

func updateLastID(cfg config.Config, lastID string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(cfg.AWSRegion),
		Credentials: credentials.NewStaticCredentials(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
	}))
	svc := dynamodb.New(sess)

	_, err := svc.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(cfg.DynamoDBLastIdsTableName),
		Item: map[string]*dynamodb.AttributeValue{
			"id":      {S: aws.String(cfg.ServiceName)},
			"last_id": {S: aws.String(lastID)},
		},
	})

	return err
}

func extractContent(doc *goquery.Document) (string, error) {
	article := doc.Find("article.item")
	if article.Length() == 0 {
		return "", fmt.Errorf("no article found")
	}

	htmlContent, err := article.Html()
	if err != nil {
		return "", fmt.Errorf("failed to get HTML content: %v", err)
	}

	converter := md.NewConverter("", true, nil)
	markdownContent, err := converter.ConvertString(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to Markdown: %v", err)
	}

	return markdownContent, nil
}

func saveToDynamoDB(cfg config.Config, message Message) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(cfg.AWSRegion),
		Credentials: credentials.NewStaticCredentials(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
	}))
	svc := dynamodb.New(sess)

	dynamoMsg := map[string]*dynamodb.AttributeValue{
		"id":         {S: aws.String(message.ID)},
		"mp":         {S: aws.String(strings.ToLower(cfg.ServiceName) + "_ms:" + message.ID)},
		"service":    {S: aws.String(message.Service)},
		"created_at": {S: aws.String(message.CreatedAt)},
		"source": {
			M: map[string]*dynamodb.AttributeValue{
				"channel":     {S: aws.String(message.Source.Channel)},
				"source_uri":  {S: aws.String(message.Source.SourceURI)},
				"sender_name": {S: aws.String(message.Source.SenderName)},
				"sender_uri":  {S: aws.String(message.Source.SenderURI)},
			},
		},
		"raw_message": {S: aws.String(message.RawMessage)},
	}

	_, err := svc.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(cfg.DynamoDBTableName),
		Item:      dynamoMsg,
	})

	return err
}

func sendMessageToSQS(cfg config.Config, message Message) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(cfg.AWSRegion),
		Credentials: credentials.NewStaticCredentials(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, ""),
	}))
	sqsSvc := sqs.New(sess)

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = sqsSvc.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(cfg.SQSQueueURL),
		MessageBody: aws.String(string(jsonMsg)),
	})

	return err
}
