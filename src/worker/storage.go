package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type dynamoPhotos struct {
	client *dynamodb.Client
	table  string
}

func (repository *dynamoPhotos) Get(ctx context.Context, id string) (photoRecord, error) {
	result, err := repository.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(repository.table), ConsistentRead: aws.Bool(true),
		Key: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
	})
	if err != nil {
		return photoRecord{}, err
	}

	if len(result.Item) == 0 {
		return photoRecord{}, errPhotoNotFound
	}

	var photo photoRecord
	err = attributevalue.UnmarshalMap(result.Item, &photo)
	return photo, err
}

func (repository *dynamoPhotos) Update(ctx context.Context, photo photoRecord, expectedStatus string) error {
	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		return err
	}

	_, err = repository.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(repository.table), Item: item,
		ConditionExpression:       aws.String("#status = :expected"),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":expected": &types.AttributeValueMemberS{Value: expectedStatus}},
	})
	var conflict *types.ConditionalCheckFailedException
	if errors.As(err, &conflict) {
		return errPhotoConflict
	}

	return err
}

type s3Photos struct {
	client         *s3.Client
	bucket         string
	maxUploadBytes int64
}

func (objects *s3Photos) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := objects.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(objects.bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	data, err := io.ReadAll(io.LimitReader(result.Body, objects.maxUploadBytes+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > objects.maxUploadBytes {
		return nil, errors.New("uploaded object exceeds the configured byte limit")
	}

	return data, nil
}

func (objects *s3Photos) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := objects.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(objects.bucket), Key: aws.String(key),
		ContentType: aws.String(contentType), Body: bytes.NewReader(data),
		CacheControl: aws.String("private, no-store"),
	})
	return err
}

func NewAWSHandler(ctx context.Context, bucket, table string, maxUploadBytes, maxImagePixels int64, places PlaceResolver) (func(context.Context, events.S3Event) error, error) {
	if bucket == "" || table == "" || maxUploadBytes <= 0 || maxImagePixels <= 0 {
		return nil, errors.New("photo bucket, table, and positive byte and pixel limits are required")
	}

	cloud, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRetryMaxAttempts(1))
	if err != nil {
		return nil, fmt.Errorf("load AWS settings: %w", err)
	}

	processor := newThumbnailWorker(
		&dynamoPhotos{client: dynamodb.NewFromConfig(cloud), table: table},
		&s3Photos{client: s3.NewFromConfig(cloud), bucket: bucket, maxUploadBytes: maxUploadBytes},
		workerConfig{Bucket: bucket, ThumbnailEdge: 640, MaxImagePixels: maxImagePixels},
		places,
	)
	return processor.HandleS3Event, nil
}
