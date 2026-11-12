package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// getSecretsPassword returns ENV["SECRETS_PASSWORD"], or else the SecureString
// parameter named by ENV["SECRETS_PASSWORD_PS_NAME"] in ENV["SECRETS_PASSWORD_PS_REGION"].
// It returns "" when neither is available, which the commands reject as a blank password.
func getSecretsPassword() string {
	if password := os.Getenv("SECRETS_PASSWORD"); len(password) > 0 {
		return password
	}

	name := os.Getenv("SECRETS_PASSWORD_PS_NAME")
	region := os.Getenv("SECRETS_PASSWORD_PS_REGION")
	if len(name) == 0 || len(region) == 0 {
		return ""
	}

	password, err := getParameter(name, region)
	if err != nil {
		fmt.Printf("[getSecretsPassword] %s\n", err)
		return ""
	}

	return password
}

func getParameter(name, region string) (string, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", err
	}

	resp, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(resp.Parameter.Value), nil
}
