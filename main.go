package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err, "Error loading .env file")
	}

	minioClient := initializeMinioClient()

	object, err := minioClient.GetObject(context.Background(), os.Getenv("S3_BUCKET"), "config.json", minio.GetObjectOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer object.Close()

	// Read the object
	config, err := io.ReadAll(object)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Print the object
	fmt.Println(string(config))

	entries := ausfallplan.GetEntriesFor(os.Getenv("LEVEL"), os.Getenv("CLASS"))

	for _, s := range entries {
		formattedDay := s.Day.Format("02.01.2006")
		fmt.Printf("%s %s %s %s\n", formattedDay, s.Hour, s.Class, s.Information)
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
