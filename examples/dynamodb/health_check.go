package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	portBytes, err := os.ReadFile(filepath.Join(os.Getenv("TMPDIR"), "@@__dynamodb:dynamodb"))
	must(err)

	client := dynamodb.New(dynamodb.Options{
		EndpointResolver: dynamodb.EndpointResolverFromURL("http://127.0.0.1:" + string(portBytes)),
		Retryer:          aws.NopRetryer{},
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "LOCALSTACK",
				SecretAccessKey: "LOCALSTACK",
			}, nil
		}),
	})

	_, err = client.ListTables(context.Background(), &dynamodb.ListTablesInput{})
	must(err)
}
