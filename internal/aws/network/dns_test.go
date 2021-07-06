package awsnetwork

import (
	"log"
	"os"
	"testing"

	cdns "github.com/unfor19/columbus-app/pkg/dns"
	"github.com/unfor19/columbus-app/pkg/traffic"
)

func testDns(r string) (string, string) {
	domainName := cdns.GetDomainName(r)
	if domainName == "" {
		return "", ""
	}
	dnsServer := "1.1.1.1:53"
	awsIpRangesFilePath := ".ip-ranges.json"
	awsIpRangesUrl := "https://ip-ranges.amazonaws.com/ip-ranges.json"
	targetIpAddress := cdns.GetTargetIPAddress(domainName, dnsServer)
	if targetIpAddress == nil {
		log.Fatalln("Failed to resolve Target IP Address")
	}
	log.Println("Target IP Address:", targetIpAddress)
	if _, err := os.Stat(awsIpRangesFilePath); os.IsNotExist(err) {
		err = traffic.DownloadFile(awsIpRangesFilePath, awsIpRangesUrl)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// Exists
		log.Println("Found AWS ip-ranges.json, skipping download:", awsIpRangesFilePath)
	}

	targetService := GetTargetAwsService(targetIpAddress.String(), awsIpRangesFilePath)
	if targetService == "" {
		return "", ""
	}

	return domainName, targetService
}

func TestCloudFrontIp(t *testing.T) {
	r := "https://dev.sokker.info"
	s := "CLOUDFRONT"
	domainName, targetAwsService := testDns(r)
	if targetAwsService != s {
		t.Fatal("Domain name", domainName, "Does not match service type", targetAwsService)
	}
}

func TestS3Ip(t *testing.T) {
	r := "https://s3.eu-west-1.amazonaws.com"
	s := "S3"
	domainName, targetAwsService := testDns(r)
	if targetAwsService != s {
		t.Fatal("Domain name", domainName, "Does not match service type", targetAwsService)
	}
}

func TestApiGatewayIp(t *testing.T) {
	r := "https://lwpcc2dff2.execute-api.eu-west-1.amazonaws.com"
	s := "API_GATEWAY"
	s2 := "EC2"
	// TODO: Find out why ApiGateway IP matches EC2 IP
	domainName, targetAwsService := testDns(r)
	if targetAwsService != s && targetAwsService != s2 {
		t.Fatal("Domain name", domainName, "Does not match service type", targetAwsService)
	}
}
