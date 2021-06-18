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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/miekg/dns"
)

var cfg aws.Config
var dnsServer string
var indexFilePath string

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
	m1.Question[0] = dns.Question{
		Name:   fmt.Sprintf(`%s.`, domainName),
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	c := new(dns.Client)
	in, _, _ := c.Exchange(m1, dnsServer)
	if t, ok := in.Answer[0].(*dns.A); ok {
		return t.A
	} else {
		return nil
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

func listCloudfrontDistributions() []types.DistributionSummary {
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
	log.Println("Found", len(distributions), "distributions")
	return distributions
}

type CloudFrontOrigin struct {
	originType                 string
	originName                 string
	originUrl                  string
	originPath                 string
	originIndexETag            string
	originBucketPolicy         s3.GetBucketPolicyOutput
	originBucketPolicyIsPublic bool
	originResourceExists       bool
	originIsWebsite            bool
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

func s3BucketExists(bucketName string) bool {
	svc := s3.NewFromConfig(cfg)
	params := s3.HeadBucketInput{
		Bucket: &bucketName,
	}
	_, err := svc.HeadBucket(context.TODO(), &params)
	if err != nil {
		// log.Println(err)
		return false
	}
	return true
}

func (o *CloudFrontOrigin) setOriginPolicy() {
	svc := s3.NewFromConfig(cfg)
	params := s3.GetBucketPolicyInput{
		Bucket: &o.originName,
	}
	resp, err := svc.GetBucketPolicy(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		o.originBucketPolicy = s3.GetBucketPolicyOutput{}
		return
	}

	o.originBucketPolicy = *resp
}

func (o *CloudFrontOrigin) s3OriginIsPublic() {
	var isPublic bool
	svc := s3.NewFromConfig(cfg)
	params := s3.GetBucketPolicyStatusInput{
		Bucket: &o.originName,
	}
	resp, err := svc.GetBucketPolicyStatus(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		isPublic = false
	} else {
		isPublic = resp.PolicyStatus.IsPublic
	}

	o.originBucketPolicyIsPublic = isPublic
}

func (o *CloudFrontOrigin) setIsBucketWebsite() {
	var isWebsite bool
	svc := s3.NewFromConfig(cfg)
	params := s3.GetBucketWebsiteInput{
		Bucket: &o.originName,
	}
	resp, err := svc.GetBucketWebsite(context.TODO(), &params)
	if err != nil {
		// log.Println(err)
		isWebsite = false
	} else {
		isWebsite = resp.ResultMetadata.Has("IndexDocument")
	}

	o.originIsWebsite = isWebsite
}

func (o *CloudFrontOrigin) setIndexETag(indexFilePath string) {
	var eTag string
	svc := s3.NewFromConfig(cfg)
	params := s3.HeadObjectInput{
		Bucket: &o.originName,
		Key:    &indexFilePath,
	}
	resp, err := svc.HeadObject(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		eTag = ""
	} else {
		eTag = strings.ReplaceAll(aws.ToString(resp.ETag), "\"", "")
	}

	o.originIndexETag = eTag
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
		o := CloudFrontOrigin{}
		o.originPath = *origin.OriginPath
		if origin.S3OriginConfig != nil {
			s3BucketName := strings.Split(*origin.DomainName, fmt.Sprintf(".s3-%s.", cfg.Region))[0]
			log.Println("Target Origin is S3 Bucket:", s3BucketName)
			o.originType = "s3-bucket"
			o.originName = s3BucketName
			o.originUrl = *origin.DomainName
			if s3BucketExists(o.originName) {
				o.setOriginPolicy()
				o.setIndexETag(indexFilePath)
				o.s3OriginIsPublic()
				o.setIsBucketWebsite()
				o.originResourceExists = true
			}
			origins = append(origins, o)
		} else if strings.Contains(aws.ToString(origin.DomainName), "s3-website") {
			log.Println("Target Origin is S3 Website:", *origin.DomainName)
			o.originType = "s3-website"
			s3BucketName := strings.Split(*origin.DomainName, ".s3-website.")[0]
			o.originName = s3BucketName
			o.originUrl = *origin.DomainName
			if s3BucketExists(o.originName) {
				o.setOriginPolicy()
				o.setIndexETag(indexFilePath)
				o.s3OriginIsPublic()
				o.setIsBucketWebsite()
				o.originResourceExists = true
			}
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
	for _, distribution := range distributions {
		if *distribution.DomainName == domainName {
			// Try with CNAME
			log.Println("Found by CNAME:", *distribution.DomainName)
			return distribution, getAwsCloudfrontOrigins(distribution)
		}
	}

	for _, distribution := range distributions {
		// Search by origins
		origins := getAwsCloudfrontOrigins(distribution)
		for _, o := range origins {
			if strings.HasPrefix(o.originUrl, domainName) {
				log.Println("Found CloudFront Distribution,", *distribution.Id, *distribution.DomainName, "by Origin", o.originName)
				return distribution, getAwsCloudfrontOrigins(distribution)
			}
		}
	}

	return types.DistributionSummary{}, nil
}

func main() {
	// TODO: set as env var
	awsRegion := "eu-west-1"
	dnsServer = "1.1.1.1:53" // Using Google's DNS Server
	var requestUrl string
	if os.Getenv("COLUMBUS_REQUEST_URL") != "" {
		// export COLUMBUS_REQUEST_URL=https://dev.sokker.info
		requestUrl = os.Getenv("COLUMBUS_REQUEST_URL")
	}

	if os.Getenv("COLUMBUS_INDEX_FILEPATH") != "" {
		indexFilePath = os.Getenv("COLUMBUS_INDEX_FILEPATH")
	} else {
		indexFilePath = "index.html"
	}

	// Find Target Service - CLOUDFRONT, S3, API_GATEWAY, EC2
	awsIpRangesFilePath := ".ip-ranges.json"
	awsIpRangesUrl := "https://ip-ranges.amazonaws.com/ip-ranges.json"
	domainName := getDomainName(requestUrl)
	log.Println("Request Domain Name:", domainName)
	targetIpAddress := getTargetIPAddress(domainName, dnsServer)
	if targetIpAddress == nil {
		log.Fatalln("Failed to resolve Target IP Address")
	}

	log.Println("Target IP Address:", targetIpAddress)
	err := DownloadFile(awsIpRangesFilePath, awsIpRangesUrl)
	if err != nil {
		log.Fatal(err)
	}
	awsIpRanges := parseAwsIpRangesFile(awsIpRangesFilePath)
	targetAwsService := getTargetAwsService(targetIpAddress.String(), awsIpRanges)
	log.Println("Target AWS Service:", targetAwsService)

	// Handle requestUrl
	requestUrlResponse := getRequestUrlResponse(requestUrl)
	log.Println("Target Url Response:")
	log.Println(requestUrlResponse.StatusCode, requestUrlResponse.Header)
	if requestUrlResponse.Header.Get("Server") == "AmazonS3" {
		requestUrlResponseEtag := strings.ReplaceAll(requestUrlResponse.Header.Get("ETag"), "\"", "")
		if requestUrlResponseEtag != "" {
			log.Println("Request Url Response ETag:", requestUrlResponseEtag)
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
	awsCloudfrontDistributions := listCloudfrontDistributions()
	targetAwsDistribution, targetOrigins := getTargetAwsCloudfrontDistribution(awsCloudfrontDistributions, domainName)
	log.Println("Target CloudFront Distribution:", *targetAwsDistribution.Id)
	log.Println("Target Distribution Status:", *targetAwsDistribution.Status)
	for i, origin := range targetOrigins {
		log.Println(i, "Origin Name:", origin.originName)
		log.Println(i, "Origin Url:", origin.originUrl)
		originUrlResponse := origin.getOriginUrlResponse()
		log.Println("Origin Response:")
		log.Println(i, "[", originUrlResponse.StatusCode, "]", originUrlResponse.Header)
		if origin.originIndexETag != "" {
			log.Println(i, "Origin ETag:", origin.originIndexETag)
		}
		if strings.HasPrefix(origin.originType, "s3-") {
			log.Println(i, "Is Resource Exists:", origin.originResourceExists)
			log.Println(i, "Is Website:", origin.originIsWebsite)
			log.Println(i, "Is Bucket Public:", origin.originBucketPolicyIsPublic)
			log.Println(i, "Origin Bucket Policy:", *origin.originBucketPolicy.Policy)
		}
	}
}
