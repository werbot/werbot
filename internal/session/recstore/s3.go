package recstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/config"
)

// S3 stores chunks in an S3-compatible bucket.
type S3 struct {
	client *s3.Client
	bucket string
}

// NewS3 builds the client: static keys when AccessKey is set, else the default AWS credential chain.
func NewS3(ctx context.Context, cfg config.RecordingsS3) (*S3, error) {
	apply := func(o *s3.Options) {
		o.Region = cfg.Region
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
		// S3-compatible servers often reject aws-chunked checksum trailers.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}
	var client *s3.Client
	if cfg.AccessKey != "" {
		opts := s3.Options{Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")}
		apply(&opts)
		client = s3.New(opts)
	} else {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}
		client = s3.NewFromConfig(awsCfg, apply)
	}
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

// Put uploads the chunk, replacing any object with the same key.
func (s *S3) Put(ctx context.Context, orgID, sessionID uuid.UUID, seq int, gz []byte) error {
	if err := checkKey(orgID, sessionID, seq); err != nil {
		return err
	}
	key := Key(orgID, sessionID, seq)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(gz),
		ContentLength: aws.Int64(int64(len(gz))),
		ContentType:   aws.String("application/gzip"),
	})
	if err != nil {
		return fmt.Errorf("recording chunk %s: %w", key, err)
	}
	return nil
}

// Get downloads the chunk; a missing object wraps fs.ErrNotExist.
func (s *S3) Get(ctx context.Context, orgID, sessionID uuid.UUID, seq int) (io.ReadCloser, error) {
	if err := checkKey(orgID, sessionID, seq); err != nil {
		return nil, err
	}
	key := Key(orgID, sessionID, seq)
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, fmt.Errorf("recording chunk %s: %w", key, fs.ErrNotExist)
		}
		return nil, fmt.Errorf("recording chunk %s: %w", key, err)
	}
	return out.Body, nil
}
