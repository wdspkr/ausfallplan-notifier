package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

type Subscription struct {
	Level string `json:"level"`
	Class string `json:"class"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err, "Error loading .env file")
	}

	minioClient := initializeMinioClient()

	subscriptions := getSubscriptions(minioClient)

	entries := ausfallplan.GetAllEntries()

	for _, subscription := range subscriptions {
		// Filter entries
		filteredEntries := ausfallplan.FilterEntries(entries, subscription.Level, subscription.Class)

		for _, s := range filteredEntries {
			formattedDay := s.Day.Format("02.01.2006")
			fmt.Printf("%s %s %s %s\n", formattedDay, s.Hour, s.Class, s.Information)
		}
	}
}

func initializeMinioClient() *minio.Client {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKeyID := os.Getenv("S3_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	useSSL := os.Getenv("S3_IS_LOCAL") != "true"

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		log.Fatal(err)
	}

	return minioClient
}

func getSubscriptions(minioClient *minio.Client) []Subscription {
	object, err := minioClient.GetObject(context.Background(), os.Getenv("S3_BUCKET"), "config.json", minio.GetObjectOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer object.Close()

	jsonConfig, err := io.ReadAll(object)
	if err != nil {
		log.Fatal(err)
	}

	subscriptions := []Subscription{}
	json.Unmarshal(jsonConfig, &subscriptions)
	return subscriptions
}
