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

	return domainName, GetTargetAwsService(targetIpAddress.String(), awsIpRangesFilePath)
}

func TestCloudFrontIp(t *testing.T) {
	r := "https://dev.sokker.info"
	domainName, targetAwsService := testDns(r)
	if targetAwsService != "CLOUDFRONT" {
		t.Fatal("Domain name", domainName, "Does not match service type", targetAwsService)
	}
}

func TestApiGatewayIp(t *testing.T) {
	r := "https://https://dev.api.sokker.info"
	domainName, targetAwsService := testDns(r)
	if targetAwsService != "CLOUDFRONT" {
		t.Fatal("Domain name", domainName, "Does not match service type", targetAwsService)
	}
}
