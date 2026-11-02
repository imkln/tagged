package api

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (repository *dynamoPhotos) Create(ctx context.Context, photo photoRecord) error {
	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		return err
	}

	_, err = repository.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(repository.table), Item: item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	return err
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

func (repository *dynamoPhotos) List(ctx context.Context) ([]photoRecord, error) {
	photos := make([]photoRecord, 0)
	pages := dynamodb.NewScanPaginator(repository.client, &dynamodb.ScanInput{
		TableName: aws.String(repository.table), ConsistentRead: aws.Bool(true),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		var records []photoRecord
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &records); err != nil {
			return nil, err
		}

		photos = append(photos, records...)
	}

	return photos, nil
}

type signedPhotoObjects struct {
	client *s3.PresignClient
	bucket string
}

func (objects *signedPhotoObjects) PresignUpload(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (string, error) {
	result, err := objects.client.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(objects.bucket), Key: aws.String(key),
		ContentType: aws.String(contentType), ContentLength: aws.Int64(size),
		IfNoneMatch: aws.String("*"),
	}, func(options *s3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func (objects *signedPhotoObjects) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	result, err := objects.client.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(objects.bucket), Key: aws.String(key),
		ResponseCacheControl: aws.String("private, no-store"),
	}, func(options *s3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func awsPhotoLibrary(ctx context.Context, config Config) (*photoLibrary, error) {
	cloud, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRetryMaxAttempts(1))
	if err != nil {
		return nil, fmt.Errorf("load AWS settings: %w", err)
	}

	if cloud.Region == "" {
		return nil, errors.New("AWS_REGION is required for photo storage")
	}

	// The browser sends the selected file; it cannot provide an SDK-computed payload checksum.
	cloud.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired

	maximum := config.MaxUploadBytes
	if maximum == 0 {
		maximum = 256 << 20
	}

	return newPhotoLibrary(
		&dynamoPhotos{client: dynamodb.NewFromConfig(cloud), table: config.PhotoTable},
		&signedPhotoObjects{client: s3.NewPresignClient(s3.NewFromConfig(cloud)), bucket: config.PhotoBucket},
		photoConfig{
			Bucket: config.PhotoBucket, MaxUploadBytes: maximum,
			UploadTTL: 5 * time.Minute, DownloadTTL: time.Minute,
			StorageOrigin: fmt.Sprintf("https://%s.s3.%s.amazonaws.com", config.PhotoBucket, cloud.Region),
		},
	), nil
}
