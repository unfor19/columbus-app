package s3

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func GetS3BucketExists(cfg aws.Config, bucketName string) bool {
	svc := s3.NewFromConfig(cfg)
	params := s3.HeadBucketInput{
		Bucket: &bucketName,
	}
	_, err := svc.HeadBucket(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}
