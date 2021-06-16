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

type CloudFrontOrigin struct {
	originType string
	originName string
	originUrl  string
	originPath string
}

func (o CloudFrontOrigin) getOriginUrlResponse() http.Response {
	if strings.ContainsAny(o.originType, "s3") {
		resp, err := http.Get("http://" + o.originUrl)
		if err != nil {
			log.Println(err)
		}
		return *resp
	}
	return http.Response{}
}

func getRequestUrlResponse(u string) http.Response {
	resp, err := http.Get(u)
	if err != nil {
		log.Println(err)
	}
	return *resp
}

func getAwsCloudfrontOrigins(distribution types.DistributionSummary) []CloudFrontOrigin {
	var origins []CloudFrontOrigin
	for _, origin := range distribution.Origins.Items {
		var o CloudFrontOrigin
		o.originPath = *origin.OriginPath
		if origin.S3OriginConfig != nil {
			s3BucketName := strings.Split(*origin.DomainName, ".s3.")[0]
			log.Println("Target Origin is S3 Bucket:", s3BucketName)
			o.originType = "s3-bucket"
			o.originName = s3BucketName
			o.originUrl = *origin.DomainName
			origins = append(origins, o)
		} else if strings.Contains(aws.ToString(origin.DomainName), "s3-website") {
			log.Println("Target Origin is S3 Website:", *origin.DomainName)
			o.originType = "s3-website"
			s3BucketName := strings.Split(*origin.DomainName, ".s3-website.")[0]
			o.originName = s3BucketName
			o.originUrl = *origin.DomainName
			origins = append(origins, o)
		}
		//  else if strings.Contains(aws.ToString(origin.DomainName), "execute-api") {
		// 	log.Println("Target Origin is API Gateway type REST:", *origin.DomainName)
		// 	originTypes = append(originTypes, "apigw-rest")
		// }
	}
	return origins
}

func getTargetAwsCloudfrontDistribution(distributions []types.DistributionSummary, domainName string) (types.DistributionSummary, []CloudFrontOrigin) {
	// TODO: Handle target origin and multiple target origins
	for _, distribution := range distributions {
		if *distribution.DomainName == domainName {
			// Try with CNAME
			log.Println("Found by CNAME:", *distribution.DomainName)
			return distribution, getAwsCloudfrontOrigins(distribution)
		} else {
			// Search by origins
			return distribution, getAwsCloudfrontOrigins(distribution)
		}
	}
	return types.DistributionSummary{}, nil
}

// func getRequestIndexEtag(requestUrl string) string {
// 	// client := &http.Client{}
// 	resp, err := http.Get(requestUrl)
// 	if err != nil {
// 		log.Fatalln(err)
// 	}
// 	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
// 		return strings.ReplaceAll(resp.Header.Get("ETag"), "\"", "")
// 	} else {
// 		return ""
// 	}
// }

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

	// Handle AWS CloudFront Distributions and their Origins
	awsCloudfrontDistributions := listCloudfrontDistributions(cfg)
	targetAwsDistribution, targetOrigins := getTargetAwsCloudfrontDistribution(awsCloudfrontDistributions, domainName)
	log.Println("Target CloudFront Distribution:", *targetAwsDistribution.Id)
	for _, origin := range targetOrigins {
		log.Println(origin.originName, origin.originUrl)
		originUrlResponse := origin.getOriginUrlResponse()
		log.Println(originUrlResponse.StatusCode, originUrlResponse.Header)
	}

	// Handle requestUrl
	requestUrlResponse := getRequestUrlResponse(requestUrl)
	log.Println(requestUrlResponse.StatusCode, requestUrlResponse.Header)
	if requestUrlResponse.Header.Get("Server") == "AmazonS3" {
		requestUrlResponseEtag := strings.ReplaceAll(requestUrlResponse.Header.Get("ETag"), "\"", "")
		if requestUrlResponseEtag != "" {
			log.Println("Request Url Response ETag:", requestUrlResponseEtag)
		}
	}
}
