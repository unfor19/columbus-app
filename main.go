package main

import (
	"fmt"
	"os"
	"regexp"
)

func getDomainName(requestUrl string) string {
	httpRegex := regexp.MustCompile(`http.*:\/\/`)
	domainName := httpRegex.ReplaceAllString(requestUrl, "")
	return domainName
}

func main() {
	// TODO: set as env var
	// awsRegion := "eu-west-1"
	requestUrl := "https://dev.sokker.info"

	domainName := getDomainName(requestUrl)
	fmt.Println(domainName)

	os.Exit(0)

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	// 	cfg, err := config.LoadDefaultConfig(context.TODO(),
	// 		config.WithRegion(awsRegion),
	// 	)
	// 	if err != nil {
	// 		log.Fatalf("unable to load SDK config, %v", err)
	// 	}

	// 	// Variables that will be used for querying
	// 	tagKey := aws.String("APP_NAME")
	// 	tagValues := []string{
	// 		"api-group",
	// 	}

	// 	// Define a tagFilter
	// 	tagFilter := types.TagFilter{}
	// 	tagFilter.Key = tagKey
	// 	tagFilter.Values = tagValues

	// 	// Using the Config value, create the Resource Groups Tagging API client
	// 	svc := resourcegroupstaggingapi.NewFromConfig(cfg)
	// 	params := &resourcegroupstaggingapi.GetResourcesInput{
	// 		TagFilters: []types.TagFilter{
	// 			// {Key: aws.String("APP_NAME"), Values: []string{"group-api"}}, // Inline values
	// 			// {Key: tagKey, Values: tagValues}, // Variables references
	// 			tagFilter, // Variable reference to an object
	// 		},
	// 	}

	// 	resp, err := svc.GetResources(context.TODO(), params)
	// 	// Build the request with its input parameters
	// 	if err != nil {
	// 		log.Fatalf("failed to list resources, %v", err)
	// 	}

	// 	// GetResources
	// 	// Returns: GetResourcesOutput { PaginationToken *string `type:"string"` , ResourceTagMappingList []*ResourceTagMapping `type:"list"` }
	// 	// Docs: https://docs.aws.amazon.com/sdk-for-go/api/service/resourcegroupstaggingapi/#GetResourcesOutput
	// 	/*
	// 		The syntax "for _, res" means we ignore the first argument of the response, in this case, ignoring PaginationToken
	// 		You should replace "_" with "pgToken" to get PaginationToken in a variable.
	// 	*/
	// 	for _, res := range resp.ResourceTagMappingList {
	// 		fmt.Println("      Pointer address:", res.ResourceARN)
	// 		fmt.Println(" Value behind pointer:", *res.ResourceARN)
	// 		fmt.Println("                Value:", aws.ToString(res.ResourceARN))
	// 		fmt.Println()
	// 	}

	// 	// Use value
	// 	random_item_index := rand.Intn(len(resp.ResourceTagMappingList))
	// 	s := "      Random Resource: " + *resp.ResourceTagMappingList[random_item_index].ResourceARN
	// 	fmt.Println(s)
}
