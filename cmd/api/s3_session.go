package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

func createS3Session(cfg config) (*session.Session, error) {
	session, err := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.s3.region),
		Endpoint: aws.String(cfg.s3.endpoint),
		Credentials: credentials.NewStaticCredentials(
			cfg.s3.access_key_id,
			cfg.s3.secret_access_key,
			"",
		),
	})
	if err != nil {
		return nil, err
	}

	return session, nil
}
