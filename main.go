package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/miekg/dns"
)

type AwsIpRangesPrefix struct {
	IpPrefix           string `json:"ip_prefix"`
	Region             string `json:"region"`
	Service            string `json:"service"`
	NetworkBorderGroup string `json:"network_border_group"`
}

type AwsIpRanges struct {
	SyncToken  string              `json:"syncToken"`
	CreateDate string              `json:"createDate"`
	Prefixes   []AwsIpRangesPrefix `json:"prefixes"`
}

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

func parseAwsIpRangesFile(filePath string) AwsIpRanges {

	file, _ := ioutil.ReadFile(filePath)

	data := AwsIpRanges{}

	_ = json.Unmarshal([]byte(file), &data)

	// for i := 0; i < len(data.Prefixes); i++ {
	// 	fmt.Println("Ip Prefix:", data.Prefixes[i].IpPrefix)
	// 	fmt.Println("Region:", data.Prefixes[i].Region)
	// 	fmt.Println("Service:", data.Prefixes[i].Service)
	// }

	return data
}

func getTargetAwsService(ip string, awsIpRanges AwsIpRanges) string {
	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		log.Fatalln("Failed to parse IP:", ip)
	}
	for _, cidr := range awsIpRanges.Prefixes {
		_, parsedCidr, _ := net.ParseCIDR(cidr.IpPrefix)
		if cidr.Service != "AMAZON" && parsedCidr.Contains(parsedIp) {
			return cidr.Service
		}
	}
	return ""
}

func listCloudfrontDistributions(cfg aws.Config) []types.DistributionSummary {
	svc := cloudfront.NewFromConfig(cfg)
	isTruncated := true
	nextMarker := aws.String("")
	params := &cloudfront.ListDistributionsInput{
		Marker:   nextMarker,
		MaxItems: aws.Int32(10),
	}
	var distributions []types.DistributionSummary
	for isTruncated == true {
		resp, err := svc.ListDistributions(context.TODO(), params)
		if err != nil {
			log.Fatalln("Failed to list for the first time:", err)
		}

		if *resp.DistributionList.IsTruncated {
			params.Marker = resp.DistributionList.NextMarker
			isTruncated = *resp.DistributionList.IsTruncated
		} else {
			isTruncated = false
		}
		// TODO: Test more than 10 distributions
		distributions = append(distributions, *&resp.DistributionList.Items...)
	}
	return distributions
}

func getTargetAwsCloudfrontDistribution(distributions []types.DistributionSummary, domainName string) types.DistributionSummary {
	for _, distribution := range distributions {
		if *distribution.DomainName == domainName {
			// Try with CNAME
			log.Println("Found by CNAME:", *distribution.DomainName)
			return distribution
		} else {
			// Search in origins
			for _, origin := range distribution.Origins.Items {
				if strings.HasPrefix(*origin.DomainName, domainName) {
					log.Println("Found by Origin:", *origin.DomainName)
					return distribution
				}
			}
		}
	}
	return types.DistributionSummary{}
}

func main() {
	// TODO: set as env var
	awsRegion := "eu-west-1"
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
	awsIpRanges := parseAwsIpRangesFile(awsIpRangesFilePath)
	targetAwsService := getTargetAwsService(targetIpAddress.String(), awsIpRanges)
	log.Println("Target AWS Service:", targetAwsService)

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsRegion),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	awsCloudfrontDistributions := listCloudfrontDistributions(cfg)
	targetAwsDistribution := getTargetAwsCloudfrontDistribution(awsCloudfrontDistributions, domainName)
	log.Println("Target CloudFront Distribution:", *targetAwsDistribution.Id)
}
