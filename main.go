package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/gin-gonic/gin"
	awsnetwork "github.com/unfor19/columbus-app/internal/aws/network"
	ccloudfront "github.com/unfor19/columbus-app/internal/aws/service/cloudfront"
	croute53 "github.com/unfor19/columbus-app/internal/aws/service/route53"
	cdns "github.com/unfor19/columbus-app/pkg/dns"
	"github.com/unfor19/columbus-app/pkg/traffic"
)

var cfg aws.Config
var dnsServer string
var indexFilePath string
var awsMapping ccloudfront.AwsMapping

func do_pipeline(requestUrl string) string {
	// TODO: set as env var
	awsRegion := "eu-west-1"
	dnsServer = "1.1.1.1:53" // Using Cloudflare's DNS Server

	if requestUrl == "" && os.Getenv("COLUMBUS_REQUEST_URL") != "" {
		// export COLUMBUS_REQUEST_URL=https://dev.sokker.info
		requestUrl = os.Getenv("COLUMBUS_REQUEST_URL")
	}

	if os.Getenv("COLUMBUS_INDEX_FILEPATH") != "" {
		indexFilePath = os.Getenv("COLUMBUS_INDEX_FILEPATH")
	} else {
		indexFilePath = "index.html"
	}

	log.Println("Request URL:", requestUrl)

	// Find Target Service - CLOUDFRONT, S3, API_GATEWAY, EC2
	awsIpRangesFilePath := ".ip-ranges.json"
	awsIpRangesUrl := "https://ip-ranges.amazonaws.com/ip-ranges.json"
	domainName := cdns.GetDomainName(requestUrl)
	awsMapping.TargetDomain.DomainName = domainName
	log.Println("Request Domain Name:", domainName)
	registeredDomainName := cdns.GetRegisteredDomainName(requestUrl)
	awsMapping.TargetDomain.RegisteredName = registeredDomainName
	log.Println("Registered Domain Name:", registeredDomainName)
	targetIpAddress := cdns.GetTargetIPAddress(domainName, dnsServer)
	if targetIpAddress == nil {
		log.Fatalln("Failed to resolve Target IP Address")
	}
	awsMapping.TargetDomain.TargetIpAddress = targetIpAddress.String()

	log.Println("Target IP Address:", targetIpAddress)
	var err error
	if _, err := os.Stat(awsIpRangesFilePath); os.IsNotExist(err) {
		err = traffic.DownloadFile(awsIpRangesFilePath, awsIpRangesUrl)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// Exists
		log.Println("Found AWS ip-ranges.json, skipping download:", awsIpRangesFilePath)
	}

	targetAwsService := awsnetwork.GetTargetAwsService(targetIpAddress.String(), awsIpRangesFilePath)
	awsMapping.TargetDomain.TargetService = targetAwsService
	log.Println("Target AWS Service:", targetAwsService)

	// Handle requestUrl
	requestUrlResponse := traffic.GetRequestUrlResponse(requestUrl)
	log.Println("Target Url Response:")
	awsMapping.TargetDomain.UrlResponse.StatusCode = requestUrlResponse.StatusCode

	log.Println(requestUrlResponse.StatusCode, requestUrlResponse.Header)
	for name, values := range requestUrlResponse.Header {
		for _, value := range values {
			awsMapping.TargetDomain.UrlResponse.Headers = append(awsMapping.TargetDomain.UrlResponse.Headers, traffic.HttpHeader{
				Name:  name,
				Value: value,
			})
		}
	}
	if requestUrlResponse.Header.Get("Server") == "AmazonS3" {
		requestUrlResponseEtag := strings.ReplaceAll(requestUrlResponse.Header.Get("ETag"), "\"", "")
		if requestUrlResponseEtag != "" {
			log.Println("Request Url Response ETag:", requestUrlResponseEtag)
			awsMapping.TargetDomain.EtagResponse = requestUrlResponseEtag
		}
	}

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err = config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsRegion),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Handle AWS CloudFront Distributions and their Origins
	awsCloudfrontDistributions := ccloudfront.ListCloudfrontDistributions(cfg)
	targetAwsDistribution, targetOrigins := ccloudfront.GetTargetAwsCloudfrontDistribution(cfg, awsCloudfrontDistributions, domainName, indexFilePath)
	log.Println("Target CloudFront Distribution:", *targetAwsDistribution.Id)
	if *targetAwsDistribution.WebACLId != "" {
		log.Println("Target CloudFront Distribution WAF Id:", *targetAwsDistribution.WebACLId)
		awsMapping.TargetDomain.WafId = *targetAwsDistribution.WebACLId
	} else {
		log.Println("Target CloudFront Distribution WAF Id:", "none")
		awsMapping.TargetDomain.WafId = "none"
	}

	log.Println("Target Distribution Status:", *targetAwsDistribution.Status)
	targetOrigins = ccloudfront.SetAwsCloudFrontOrigins(cfg, targetOrigins)
	route53Record := croute53.GetRoute53Record(cfg, requestUrl, domainName)
	log.Println("Route53 record:", route53Record)
	ips, err := net.LookupIP(domainName + ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
		os.Exit(1)
	}
	for _, ip := range ips {
		awsMapping.TargetDomain.NsLookup = append(awsMapping.TargetDomain.NsLookup, domainName+"."+" IN A "+ip.String()+"\n")
	}
	awsMapping.TargetDomain.Route53Record = route53Record
	awsMapping.CloudFrontOrigins = targetOrigins
	b, err := json.Marshal(awsMapping)
	if err != nil {
		log.Fatalln(err)
	}
	return string(b)
}

func getExplore(c *gin.Context) {
	requestUrl := c.Query("requestUrl")
	response := do_pipeline(requestUrl)
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(response))
	awsMapping = ccloudfront.AwsMapping{}
}

func main() {
	log.Println("Starting server ...")
	if os.Getenv("GO_GIN_DEBUG") != "true" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.GET("/explore", getExplore)

	r.GET("/", func(c *gin.Context) {
		response := "Use the following query http://localhost:8080/explore?requestUrl=https://dev.api.sokker.info"
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Data(200, "Content-Type: text/plain; charset=utf-8", []byte(response))
	})
	r.Run(":8080") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
