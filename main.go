package main

import (
	"errors"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/aws"
)

// VaultConfig is for vault interface
type VaultConfig struct {
	vaultAddr    string
	token        string
	snapshotPath string
	insecure     bool
}

// AWSConfig is for aws interaction
type AWSConfig struct {
	s3Bucket string
	s3Prefix string
	s3Region string
}

func main() {
	// initialize vaultConfig and awsConfig
	insecure, err := strconv.ParseBool(os.Getenv("VAULT_SKIP_VERIFY"))
	if err != nil {
		log.Fatalln("Invalid boolean value for VAULT_SKIP_VERIFY")
	}
	vaultConfig := VaultConfig{
		vaultAddr:    os.Getenv("VAULT_ADDR"),
		token:        os.Getenv("VAULT_TOKEN"),
		snapshotPath: os.Getenv("VAULT_SNAPSHOT_PATH"),
		insecure:     insecure,
	}
	awsConfig := AWSConfig{
		s3Bucket: os.Getenv("S3_BUCKET"),
		s3Prefix: os.Getenv("S3_PREFIX"),
		s3Region: os.Getenv("AWS_REGION"),
	}

	// initialize and configure client
  vaultClient, err := vaultClientConfig(&vaultConfig)
	if err != nil {
		log.Fatalln("Vault client initialization and configuration failed")
	}

	// vault raft snapshot
	snapshotFile, err := vaultRaftSnapshot(vaultClient, vaultConfig.snapshotPath)
	if err != nil {
		log.Fatalln("Vault Raft Snapshot failed")
	}

	// initialize awsConfig
	uploadResult, err := snapshotS3Upload(&awsConfig, snapshotFile.Name())
	if err != nil {
		log.Fatalln("S3 upload failed")
	}

	// output info
	fmt.Printf("Vault Raft snapshot uploaded to, %s\n", aws.StringValue(&uploadResult.Location))
}

// vault client configuration
func vaultClientConfig(config *VaultConfig) (*vault.Client, error) {
	// initialize config
	vaultConfig := &vault.Config{Address: config.vaultAddr}
	err := vaultConfig.ConfigureTLS(&vault.TLSConfig{Insecure: config.insecure})
	if err != nil {
		fmt.Println("Vault TLS configuration failed to initialize")
		fmt.Println(err)
		return nil, err
	}

	// initialize client
	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		fmt.Println("Vault client failed to initialize")
		fmt.Println(err)
		return nil, err
	}

	// determine authentication method
	if config.token == "aws-iam" {
		// authenticate with aws iam
		awsAuth, err := auth.NewAWSAuth(auth.WithIAMAuth())
		if err != nil {
			return nil, errors.New("Unable to initialize AWS IAM authentication")
		}

		authInfo, err := client.Auth().Login(context.TODO(), awsAuth)
		if err != nil {
			return nil, errors.New("Unable to login to AWS IAM auth method")
		}
		if authInfo == nil {
			return nil, errors.New("No auth info was returned after login")
		}
	} else {
		// authenticate with token
		if len(config.token) != 26 {
			return nil, errors.New("The Vault token is invalid")
		}
		client.SetToken(config.token)
	}

	// return vault client interface
  return client, nil
}

// vault raft snapshot creation
func vaultRaftSnapshot(client *vault.Client, snapshotPath string) (*os.File, error) {
	// prepare snaptshot file
	snapshotFile, err := os.OpenFile(snapshotPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Println("snapshot file at " + snapshotPath + " could not be created")
		fmt.Println(err)
		return nil, err
	}

	// defer snapshot close
	defer snapshotFileClose(snapshotFile)

	// execute raft snapshot
	err = client.Sys().RaftSnapshot(snapshotFile)
	if err != nil {
		snapshotFile.Close()
		fmt.Println("Vault Raft snapshot invocation failed")
		fmt.Println(err)
		return nil, err
	}

	return snapshotFile, nil
}

// snapshot upload to s3
func snapshotS3Upload(config *AWSConfig, snapshotPath string) (*s3manager.UploadOutput, error) {
	// open snapshot and defer closing
	snapshotFile, err := os.Open(snapshotPath)
	if err != nil {
		fmt.Printf("Failed to open snapshot file %q: %v", snapshotPath, err)
		return nil, err
	}
	defer snapshotFileClose(snapshotFile)

	// aws session
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(config.s3Region),
	}))

	// initialize an uploader with the session and default options
	uploader := s3manager.NewUploader(awsSession)

	// determine vault backup base for s3 key
	snapshotPathBase := filepath.Base(snapshotPath)

	// upload the snapshot to the s3bucket at specified key
	uploadResult, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(config.s3Bucket),
		Key:    aws.String(config.s3Prefix + "-" + snapshotPathBase),
		Body:   snapshotFile,
	})
	if err != nil {
		fmt.Println("Vault backup failed to upload to S3 bucket " + config.s3Bucket)
		fmt.Println(err)
		return nil, err
	}

	return uploadResult, nil
}

// close snapshot file
func snapshotFileClose(snapshotFile *os.File) {
	// close file
	err := snapshotFile.Close()
	if err != nil {
		fmt.Println("Vault raft snapshot file failed to close")
		log.Fatalln(err)
	}
}
