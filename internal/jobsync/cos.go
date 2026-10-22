package jobsync

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"mrolescope/internal/config"
)

// COSStore uses Tencent COS's S3-compatible API. Endpoint is the regional
// service endpoint (without a bucket); the SDK selects the bucket hostname.
type COSStore struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

func NewCOSStore(ctx context.Context, cfg config.COSConfig) (*COSStore, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load COS client configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = false
	})
	return &COSStore{client: client, bucket: cfg.Bucket, endpoint: cfg.Endpoint}, nil
}

func (store *COSStore) Put(ctx context.Context, key string, body io.Reader, size int64, metadata map[string]string) error {
	_, err := store.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(store.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		Metadata:      metadata,
	})
	return err
}

func (store *COSStore) Head(ctx context.Context, key string) (ObjectInfo, error) {
	result, err := store.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)})
	if err != nil {
		return ObjectInfo{}, err
	}
	metadata := make(map[string]string, len(result.Metadata))
	for name, value := range result.Metadata {
		metadata[strings.ToLower(name)] = value
	}
	return ObjectInfo{Size: aws.ToInt64(result.ContentLength), Metadata: metadata}, nil
}

func (store *COSStore) URI(key string) string {
	return "s3://" + store.bucket + "/" + key
}
