package utils

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func UploadToSpaces(file io.Reader, filename string, contentType string) (string, error) {
	key := os.Getenv("SPACES_KEY")
	secret := os.Getenv("SPACES_SECRET")
	region := os.Getenv("SPACES_REGION")
	bucket := os.Getenv("SPACES_BUCKET")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")),
	)
	if err != nil {
		log.Printf("Error loading AWS config: %v", err)
		return "", err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.digitaloceanspaces.com", region))
	})

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(filename),
		Body:        file,
		ContentType: aws.String(contentType),
		ACL:         types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		log.Printf("Error uploading to Spaces (bucket=%s, key=%s): %v", bucket, filename, err)
		return "", err
	}

	url := fmt.Sprintf("https://%s.%s.cdn.digitaloceanspaces.com/%s", bucket, region, filename)
	return url, nil
}

func DeleteFromSpaces(fileURL string) error {
	key := os.Getenv("SPACES_KEY")
	secret := os.Getenv("SPACES_SECRET")
	region := os.Getenv("SPACES_REGION")
	bucket := os.Getenv("SPACES_BUCKET")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")),
	)
	if err != nil {
		log.Printf("Error loading AWS config: %v", err)
		return err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.digitaloceanspaces.com", region))
	})

	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return err
	}
	objectKey := strings.TrimPrefix(parsedURL.Path, "/")

	_, err = client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Error deleting from Spaces (bucket=%s, key=%s): %v", bucket, objectKey, err)
		return err
	}

	return nil
}
