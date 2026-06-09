package application

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Config holds the configuration for DigitalOcean Spaces / S3
type S3Config struct {
	BucketName   string
	Region       string
	AccessKey    string
	SecretKey    string
	Endpoint     string
	RootFolder   string
	UploadExpiry int
}

// S3Service handles uploading and fetching files to/from S3/Spaces
type S3Service struct {
	Config *S3Config
	client *s3.S3
}

// NewS3Service creates a new S3Service instance
func NewS3Service(config *S3Config) (*S3Service, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(config.Region),
		Credentials:      credentials.NewStaticCredentials(config.AccessKey, config.SecretKey, ""),
		Endpoint:         aws.String(config.Endpoint),
		S3ForcePathStyle: aws.Bool(false), // DO Spaces uses virtual-hosted style
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3Service{
		Config: config,
		client: s3.New(sess),
	}, nil
}

// UploadFile uploads a file directly to the configured S3 bucket
func (s *S3Service) UploadFile(key string, contentType string, fileBody io.ReadSeeker) error {
	_, err := s.client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.Config.BucketName),
		Key:         aws.String(key),
		Body:        fileBody,
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}
	return nil
}

// GetFile fetches a file's read stream and content type from the S3 bucket
func (s *S3Service) GetFile(key string) (io.ReadCloser, string, error) {
	resp, err := s.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.Config.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch file from S3: %w", err)
	}

	contentType := ""
	if resp.ContentType != nil {
		contentType = *resp.ContentType
	} else {
		extension := filepath.Ext(key)
		if extension == ".pdf" {
			contentType = "application/pdf"
		} else {
			contentType = "application/octet-stream"
		}
	}

	return resp.Body, contentType, nil
}
