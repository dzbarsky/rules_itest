package main

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	cmd := exec.Command(os.Getenv("GET_ASSIGNED_PORT_BIN"), "@@//dynamodb:dynamodb")
	port, err := cmd.CombinedOutput()
	must(err)

	client := dynamodb.New(dynamodb.Options{
		EndpointResolver: dynamodb.EndpointResolverFromURL("http://127.0.0.1:" + string(port)),
		Retryer:          aws.NopRetryer{},
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "LOCALSTACK",
				SecretAccessKey: "LOCALSTACK",
			}, nil
		}),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		os.Exit(1)
	}
	must(err)
}
