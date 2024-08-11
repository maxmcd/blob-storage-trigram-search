package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Panicln("Error loading .env file")
	}
	minioClient, err := minio.New(os.Getenv("AWS_ENDPOINT_URL_S3"), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
		Secure: true,
	})
	if err != nil {
		log.Panicln(err)
	}
	fmt.Println(minioClient.BucketExists(context.Background(), "s3-trigram-search"))

}
