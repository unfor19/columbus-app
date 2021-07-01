package awsnetwork

import (
	"log"
	"os"
	"testing"

	cdns "github.com/unfor19/columbus-app/pkg/dns"
	"github.com/unfor19/columbus-app/pkg/traffic"
)

func TestAlbIp(t *testing.T) {
	r := os.Getenv("REQUEST_URL")
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

	targetAwsService := GetTargetAwsService(targetIpAddress.String(), awsIpRangesFilePath)
	log.Println("Target service:", targetAwsService)
	if targetAwsService != "EC2" {
		t.Fatal("Domain name", domainName, "Does not match service type", targetAwsService)
	}
}
