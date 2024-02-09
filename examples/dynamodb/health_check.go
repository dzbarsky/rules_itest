package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// TODO(zbarsky): Would be a bit nicer to provide MustPort as an svclib library
	ports := map[string]string{}
	err := json.Unmarshal([]byte(os.Getenv("ASSIGNED_PORTS")), &ports)
	must(err)

	port := ports["@@//dynamodb:dynamodb"]

	client := dynamodb.New(dynamodb.Options{
		EndpointResolver: dynamodb.EndpointResolverFromURL("http://127.0.0.1:" + port),
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
