package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

func getDomainName(requestUrl string) string {
	httpRegex := regexp.MustCompile(`^http.*:\/\/`)
	domainName := httpRegex.ReplaceAllString(requestUrl, "")
	return domainName
}

func getTargetIPAddress(domainName string, dnsServer string) net.IP {
	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.RecursionDesired = true
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{fmt.Sprintf(`%s.`, domainName), dns.TypeA, dns.ClassINET}
	c := new(dns.Client)
	in, _, _ := c.Exchange(m1, dnsServer)
	if t, ok := in.Answer[0].(*dns.A); ok {
		return t.A
	} else {
		return nil
	}
}

func getCidr(cidrType string, ipAddress string) string {
	cidrs := strings.Split(ipAddress, ".")
	switch d := cidrType; d {
	case "A":
		return cidrs[0]
	case "B":
		return cidrs[1]
	case "C":
		return cidrs[2]
	case "D":
		return cidrs[3]
	default:
		return ""
	}
}

// Source: https://golangcode.com/download-a-file-from-a-url/
// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filepath string, url string) error {
	log.Println("Downloading the file:", url)
	log.Println("Filepath:", filepath)
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func main() {
	// TODO: set as env var
	// awsRegion := "eu-west-1"
	requestUrl := "https://dev.sokker.info"
	awsIpRangesFilePath := ".ip-ranges.json"
	awsIpRangesUrl := "https://ip-ranges.amazonaws.com/ip-ranges.json"

	domainName := getDomainName(requestUrl)
	log.Println("Request Domain Name:", domainName)
	dnsServer := "8.8.8.8:53"
	targetIpAddress := getTargetIPAddress(domainName, dnsServer)
	log.Println("Target IP Address:", targetIpAddress)
	targetCidrA := getCidr("A", string(targetIpAddress.String()))
	log.Println("Target Cidr A:", targetCidrA)
	err := DownloadFile(awsIpRangesFilePath, awsIpRangesUrl)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Downloaded complete")
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
