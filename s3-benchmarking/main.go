package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"strconv"
)

func main() {
	_, err := runCommand("cf", "api", os.Getenv("CF_API"), "--skip-ssl-validation")
	if err != nil {
		log.Fatalf("cf api failed: %s", err.Error())
	}

	_, err = runCommand("cf", "auth", "admin", os.Getenv("CF_ADMIN_PASSWORD"))
	if err != nil {
		log.Fatalf("cf auth failed: %s", err.Error())
	}

	_, err = runCommand("cf", "create-org", os.Getenv("CF_ORG"))
	if err != nil {
		log.Fatalf("cf create-org failed: %s", err.Error())
	}

	_, err = runCommand("cf", "target", "-o", "test")
	if err != nil {
		log.Fatalf("cf target org failed: %s", err.Error())
	}

	_, err = runCommand("cf", "create-space", os.Getenv("CF_SPACE"))
	if err != nil {
		log.Fatalf("cf create-space failed: %s", err.Error())
	}

	_, err = runCommand("cf", "target", "-s", "test")
	if err != nil {
		log.Fatalf("cf target space failed: %s", err.Error())
	}

	numberOfApps, err := strconv.Atoi(os.Getenv("NUMBER_OF_APPS"))
	if err != nil {
		log.Fatalf("failed to parse NUMBER_OF_APPS: %s", err.Error())
	}

	for i := 0; ; i++ {
		fmt.Printf("Push #%d\n", i+1)

		startTime := time.Now().Unix()
		_, err = runCommand("cf", "push", "-p", os.Getenv("APP_PATH"), "-b", "staticfile_buildpack", "-m", "64M", fmt.Sprintf("big-app-%d", i%numberOfApps+1))
		if err != nil {
			log.Printf("cf push failed: %s", err.Error())
		}
		finishTime := time.Now().Unix()
		pushDuration := finishTime - startTime

		blobstoreSize, err := getBucketSizes(
			os.Getenv("DROPLETS_BUCKET"),
			os.Getenv("BUILDBPACKS_BUCKET"),
			os.Getenv("PACKAGES_BUCKET"),
			os.Getenv("RESOURCES_BUCKET"),
		)
		if err != nil {
			log.Fatalf("Getting blobstore size failed: %s", err.Error())
		}

		fmt.Printf("blobstore size: %dMB\n", blobstoreSize.Megabytes)
		fmt.Printf("blobstore number of objects: %d\n", blobstoreSize.NumOfFiles)
		fmt.Printf("cf push duration: %ds\n", pushDuration)

		db, err := sql.Open("mysql", fmt.Sprintf("root:%s@tcp(%s:3306)/%s", os.Getenv("MYSQL_ROOT_PASSWORD"), os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_DATABASE")))
		_, err = db.Exec(
			fmt.Sprintf(
				"INSERT INTO %s (blobstore_size, timestamp, cf_push_time, blobstore_num_of_files) VALUES (%d, %d, %d, %d)",
				os.Getenv("MYSQL_DATABASE_TABLE"),
				blobstoreSize.Megabytes,
				startTime,
				pushDuration,
				blobstoreSize.NumOfFiles,
			),
		)
		if err != nil {
			log.Fatalf("Insert into db failed: %s", err.Error())
		}
	}
}

type BucketSize struct {
	Megabytes  int
	NumOfFiles int
}

func getBucketSizes(buckets ...string) (BucketSize, error) {
	var totalMegabytes = 0
	var totalNumOfFiles = 0

	for _, bucket := range buckets {
		bucketSize, err := getBucketSize(bucket)
		if err != nil {
			return BucketSize{}, err
		}

		totalMegabytes += bucketSize.Megabytes
		totalNumOfFiles += bucketSize.NumOfFiles
	}

	return BucketSize{Megabytes: totalMegabytes, NumOfFiles: totalNumOfFiles}, nil
}

func getBucketSize(bucket string) (BucketSize, error) {
	var args []string
	endpoint, found := os.LookupEnv("S3_ENDPOINT")

	if found {
		args = append(args, "--url-endpoint", endpoint)
	}

	args = append(args,
		"--region", "eu-west-1",
		"s3api",
		"list-object-versions",
		"--bucket", bucket,
		"--output", "json",
		"--query", "[sum(Versions[].Size), length(Versions[])]")

	outputBuffer, err := runCommand("aws", args...)
	if err != nil {
		return BucketSize{}, err
	}

	var response []int
	err = json.Unmarshal(outputBuffer.Bytes(), &response)
	if err != nil {
		return BucketSize{}, err
	}

	return BucketSize{Megabytes: response[0] / 1024 / 1024, NumOfFiles: response[1]}, nil
}

func runCommand(cmd string, args ...string) (*bytes.Buffer, error) {
	outputBuffer := new(bytes.Buffer)
	errorBuffer := new(bytes.Buffer)

	awsCmd := exec.Command(cmd, args...)
	awsCmd.Stdout = outputBuffer
	awsCmd.Stderr = errorBuffer

	err := awsCmd.Run()
	if err != nil {
		return nil, fmt.Errorf(string(errorBuffer.Bytes()))
	}

	return outputBuffer, err
}
