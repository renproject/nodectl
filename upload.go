package nodectl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/fatih/color"
	"github.com/renproject/nodectl/renvm"
	"github.com/urfave/cli/v2"
)

var (
	BucketRegion = "ap-southeast-1"

	BucketName = "darknode.renproject.io"
)

func Upload(ctx *cli.Context) error {
	config := ctx.String("config")
	snapshot := ctx.String("snapshot")
	network := ctx.String("network")
	accessKey := ctx.String("aws-access-key")
	secretKey := ctx.String("aws-secret-key")

	// Some validation for input arguments
	if snapshot == "" && config == "" {
		return errors.New("nothing to upload")
	}
	var err error
	if config != "" {
		config, err = filepath.Abs(config)
		if err != nil {
			return fmt.Errorf("invalid config file path, err = %v", err)
		}
	}
	if snapshot != "" {
		snapshot, err = filepath.Abs(snapshot)
		if err != nil {
			return fmt.Errorf("invalid snapshot path, err = %v", err)
		}
	}

	// Try reading the default credential file if user does not provide credentials directly
	if accessKey == "" || secretKey == "" {
		cred := credentials.NewSharedCredentials("", ctx.String("aws-profile"))
		credValue, err := cred.Get()
		if err != nil {
			return errors.New("invalid credentials")
		}
		accessKey, secretKey = credValue.AccessKeyID, credValue.SecretAccessKey
		if accessKey == "" || secretKey == "" {
			return errors.New("invalid credentials")
		}
	}

	// initialise AWS uploader
	cred := credentials.NewStaticCredentials(accessKey, secretKey, "")
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(BucketRegion),
		Credentials: cred,
	})
	if err != nil {
		return err
	}
	uploader := s3manager.NewUploader(sess)

	if config != "" {
		color.Yellow("- Uploading config file...")
		if err := uploadConfig(config, network, uploader); err != nil {
			return err
		}
		color.Green("- Successfully uploaded config file")
	}

	if snapshot != "" {
		color.Yellow("- Uploading snapshot...")
		if err := uploadSnapshot(config, network, uploader); err != nil {
			return err
		}
		color.Green("- Successfully uploaded snapshot")
	}

	return nil
}

func uploadConfig(filePath, network string, uploader *s3manager.Uploader) error {
	// Validate the file is a valid config file
	_, err := renvm.NewOptionsFromFile(filePath)
	if err != nil {
		return err
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Upload the file
	acl := "public-read"
	_, err = uploader.Upload(&s3manager.UploadInput{
		ACL:    &acl,
		Bucket: aws.String(BucketName),
		Key:    aws.String(fmt.Sprintf("/%v/config.json", network)),
		Body:   file,
	})
	return err
}

func uploadSnapshot(snapshotPath, network string, uploader *s3manager.Uploader) error {
	// Make sure the format is '.tar.gz'
	if !strings.HasSuffix(snapshotPath, ".tar.gz") {
		return fmt.Errorf("invalid snapshot format")
	}

	// Open the file
	file, err := os.Open(snapshotPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Upload the file
	acl := "public-read"
	_, err = uploader.Upload(&s3manager.UploadInput{
		ACL:    &acl,
		Bucket: aws.String(BucketName),
		Key:    aws.String(fmt.Sprintf("/%v/latest.tar.gz", network)),
		Body:   file,
	})
	return err
}
