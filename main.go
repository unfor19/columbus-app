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
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
)

var cfg aws.Config
var dnsServer string
var indexFilePath string
var awsMapping AwsMapping

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

type AwsResource struct {
	Arn          string
	Id           string
	Name         string
	ResourceType string
}

type HttpHeader struct {
	Name  string
	Value string
}

type UrlResponse struct {
	StatusCode int
	Headers    []HttpHeader
}

type TargetAttributes struct {
	DomainName      string
	RegisteredName  string
	TargetIpAddress string
	TargetService   string
	UrlResponse     UrlResponse
	EtagResponse    string
	Route53Record   string
	WafId           string
	NsLookup        []string
}

type AwsMapping struct {
	CloudFrontOrigins []CloudFrontOrigin
	TargetDomain      TargetAttributes
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

type Principal struct {
	AWS string
}

type StatementEntry struct {
	Sid       string
	Effect    string
	Action    string
	Resource  string
	Principal Principal
}

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
	Id        string
}

type CloudFrontOrigin struct {
	OriginType                 string
	OriginName                 string
	OriginUrl                  string
	OriginPath                 string
	OriginIndexETag            string
	originBucketPolicy         string
	OriginBucketPolicy         PolicyDocument
	OriginBucketPolicyIsPublic bool
	OriginResourceExists       bool
	OriginIsWebsite            bool
	OriginUrlResponse          UrlResponse
}

func (o CloudFrontOrigin) getOriginUrlResponse() http.Response {
	var resp http.Response
	if strings.ContainsAny(o.OriginType, "s3") {
		log.Println("s3", o.OriginUrl)
		resp, err := http.Get("http://" + o.OriginUrl)
		if err != nil {
			log.Println(err)
		}
		return *resp
	} else if strings.ContainsAny(o.OriginUrl, "execute-api") {
		log.Println("apigw", o.OriginUrl)
		resp, err := http.Get("https://" + o.OriginUrl + "/" + o.OriginPath)
		if err != nil {
			log.Println(err)
		}
		return *resp
	} else {
		log.Println("unknown", o.OriginUrl)
		log.Fatalln("Unknown origin type", o.OriginType)
	}
	return resp
}

func (o *CloudFrontOrigin) setOriginUrlResponse(resp http.Response) {
	for name, values := range resp.Header {
		for _, value := range values {
			o.OriginUrlResponse.Headers = append(o.OriginUrlResponse.Headers, HttpHeader{
				Name:  name,
				Value: value,
			})
		}
	}
	o.OriginUrlResponse.StatusCode = resp.StatusCode
}

func s3BucketExists(bucketName string) bool {
	svc := s3.NewFromConfig(cfg)
	params := s3.HeadBucketInput{
		Bucket: &bucketName,
	}
	_, err := svc.HeadBucket(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func (o *CloudFrontOrigin) setOriginPolicy() {
	svc := s3.NewFromConfig(cfg)
	params := s3.GetBucketPolicyInput{
		Bucket: &o.OriginName,
	}
	resp, err := svc.GetBucketPolicy(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		o.originBucketPolicy = "none"
		return
	}

	o.originBucketPolicy = *resp.Policy
}

func (o *CloudFrontOrigin) s3OriginIsPublic() {
	var isPublic bool
	svc := s3.NewFromConfig(cfg)
	params := s3.GetBucketPolicyStatusInput{
		Bucket: &o.OriginName,
	}
	resp, err := svc.GetBucketPolicyStatus(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		isPublic = false
	} else {
		isPublic = resp.PolicyStatus.IsPublic
	}

	o.OriginBucketPolicyIsPublic = isPublic
}

func (o *CloudFrontOrigin) setIsBucketWebsite() {
	var isWebsite bool
	svc := s3.NewFromConfig(cfg)
	params := s3.GetBucketWebsiteInput{
		Bucket: &o.OriginName,
	}
	resp, err := svc.GetBucketWebsite(context.TODO(), &params)
	if err != nil {
		isWebsite = false
	} else {
		isWebsite = resp.ResultMetadata.Has("IndexDocument")
	}

	o.OriginIsWebsite = isWebsite
}

func (o *CloudFrontOrigin) setIndexETag(indexFilePath string) {
	var eTag string
	svc := s3.NewFromConfig(cfg)
	params := s3.HeadObjectInput{
		Bucket: &o.OriginName,
		Key:    &indexFilePath,
	}
	resp, err := svc.HeadObject(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		eTag = ""
	} else {
		eTag = strings.ReplaceAll(aws.ToString(resp.ETag), "\"", "")
	}

	o.OriginIndexETag = eTag
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
		o.OriginPath = *origin.OriginPath
		o.OriginUrl = *origin.DomainName
		log.Println("Origin Domain Name", o.OriginUrl)
		if origin.S3OriginConfig != nil {
			s3BucketName := strings.Split(*origin.DomainName, fmt.Sprintf(".s3.%s.", cfg.Region))[0]
			log.Println("Target Origin is S3 Bucket:", s3BucketName)
			o.OriginType = "s3-bucket"
			o.OriginName = s3BucketName
			if s3BucketExists(o.OriginName) {
				o.setOriginPolicy()
				o.setIndexETag(indexFilePath)
				o.s3OriginIsPublic()
				o.setIsBucketWebsite()
				o.OriginResourceExists = true
			}
			origins = append(origins, o)
		} else if origin.CustomOriginConfig != nil && strings.Contains(aws.ToString(origin.DomainName), "s3-website") {
			log.Println("Target Origin is S3 Website:", o.OriginUrl)
			o.OriginType = "s3-website"
			s3BucketName := strings.Split(o.OriginUrl, ".s3-website.")[0]
			o.OriginName = s3BucketName
			if s3BucketExists(o.OriginName) {
				o.setOriginPolicy()
				o.setIndexETag(indexFilePath)
				o.s3OriginIsPublic()
				o.setIsBucketWebsite()
				o.OriginResourceExists = true
			}
			origins = append(origins, o)
		} else if strings.ContainsAny(aws.ToString(origin.DomainName), ".execute-api.") {
			log.Println("Target Origin is API Gateway type REST:", o.OriginUrl)
			o.OriginType = "apigw"
			apigwName := strings.Split(o.OriginUrl, ".execute-api.")[0]
			o.OriginName = apigwName
			origins = append(origins, o)
		}
	}
	return origins
}

func getTargetAwsCloudfrontDistribution(distributions []types.DistributionSummary, domainName string) (types.DistributionSummary, []CloudFrontOrigin) {
	for _, distribution := range distributions {
		// Search by origins
		origins := getAwsCloudfrontOrigins(distribution)
		for _, o := range origins {
			if strings.HasPrefix(o.OriginUrl, domainName) || strings.Contains(o.OriginUrl, ".execute-api.") {
				log.Println("Found CloudFront Distribution,", *distribution.Id, *distribution.DomainName, "by Origin", o.OriginName)
				return distribution, origins
			}
		}
	}

	return types.DistributionSummary{}, nil
}

func getRegisteredDomainName(requestUrl string) string {
	u, err := url.Parse(requestUrl)
	if err != nil {
		log.Fatal(err)
	}
	parts := strings.Split(u.Hostname(), ".")
	domain := parts[len(parts)-2] + "." + parts[len(parts)-1]
	return domain
}

func getRoute53Record(requestUrl string, domainName string) string {
	registeredDomainName := getRegisteredDomainName(requestUrl)
	params := route53.ListHostedZonesByNameInput{
		DNSName: &registeredDomainName,
	}
	svc := route53.NewFromConfig(cfg)
	resp, err := svc.ListHostedZonesByName(context.TODO(), &params)
	if err != nil {
		log.Println(err)
		return "none"
	}

	if len(resp.HostedZones) == 1 && strings.Contains(*resp.HostedZones[0].Name, registeredDomainName) {
		hostedZoneId := resp.HostedZones[0].Id
		params := route53.ListResourceRecordSetsInput{
			HostedZoneId: hostedZoneId,
		}
		resp, err := svc.ListResourceRecordSets(context.TODO(), &params)
		if err != nil {
			log.Fatalln(err)
		}

		for _, r := range resp.ResourceRecordSets {
			if fmt.Sprint(domainName, ".") == *r.Name {
				return *r.Name
			}
		}
	}

	return "none"
}

type RequestUrlResponse struct {
	statusCode int
	body       string
	headers    http.Header
}

type ColumbusOutput struct {
	awsRegion        string
	requestUrl       string
	domainName       string
	registeredDomain string
	targetIpAddress  string
}

func do_pipeline(requestUrl string) string {
	// TODO: set as env var
	awsRegion := "eu-west-1"
	dnsServer = "1.1.1.1:53" // Using Google's DNS Server

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
	domainName := getDomainName(requestUrl)
	awsMapping.TargetDomain.DomainName = domainName
	log.Println("Request Domain Name:", domainName)
	registeredDomainName := getRegisteredDomainName(requestUrl)
	awsMapping.TargetDomain.RegisteredName = registeredDomainName
	log.Println("Registered Domain Name:", registeredDomainName)
	targetIpAddress := getTargetIPAddress(domainName, dnsServer)
	if targetIpAddress == nil {
		log.Fatalln("Failed to resolve Target IP Address")
	}
	awsMapping.TargetDomain.TargetIpAddress = targetIpAddress.String()

	log.Println("Target IP Address:", targetIpAddress)
	var err error
	if _, err := os.Stat(awsIpRangesFilePath); os.IsNotExist(err) {
		err = DownloadFile(awsIpRangesFilePath, awsIpRangesUrl)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// Exists
		log.Println("Found AWS ip-ranges.json, skipping download:", awsIpRangesFilePath)
	}
	awsIpRanges := parseAwsIpRangesFile(awsIpRangesFilePath)
	targetAwsService := getTargetAwsService(targetIpAddress.String(), awsIpRanges)
	awsMapping.TargetDomain.TargetService = targetAwsService
	log.Println("Target AWS Service:", targetAwsService)

	// Handle requestUrl
	requestUrlResponse := getRequestUrlResponse(requestUrl)
	log.Println("Target Url Response:")
	awsMapping.TargetDomain.UrlResponse.StatusCode = requestUrlResponse.StatusCode

	log.Println(requestUrlResponse.StatusCode, requestUrlResponse.Header)
	for name, values := range requestUrlResponse.Header {
		for _, value := range values {
			awsMapping.TargetDomain.UrlResponse.Headers = append(awsMapping.TargetDomain.UrlResponse.Headers, HttpHeader{
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
	awsCloudfrontDistributions := listCloudfrontDistributions()
	targetAwsDistribution, targetOrigins := getTargetAwsCloudfrontDistribution(awsCloudfrontDistributions, domainName)
	log.Println("Target CloudFront Distribution:", *targetAwsDistribution.Id)
	if *targetAwsDistribution.WebACLId != "" {
		log.Println("Target CloudFront Distribution WAF Id:", *targetAwsDistribution.WebACLId)
		awsMapping.TargetDomain.WafId = *targetAwsDistribution.WebACLId
	} else {
		log.Println("Target CloudFront Distribution WAF Id:", "none")
		awsMapping.TargetDomain.WafId = "none"
	}

	log.Println("Target Distribution Status:", *targetAwsDistribution.Status)
	for i, origin := range targetOrigins {
		log.Println(i, "Origin Type:", origin.OriginType)
		log.Println(i, "Origin Name:", origin.OriginName)
		log.Println(i, "Origin Url:", origin.OriginUrl)
		originUrlResponse := origin.getOriginUrlResponse()
		targetOrigins[i].setOriginUrlResponse(originUrlResponse)
		var bucketPolicy PolicyDocument
		policyBlob := []byte(targetOrigins[i].originBucketPolicy)
		err := json.Unmarshal(policyBlob, &bucketPolicy)
		if err != nil {
			log.Println(err)
		}
		targetOrigins[i].OriginBucketPolicy = bucketPolicy
		// log.Println("Origin Response:")
		// log.Println(i, "[", originUrlResponse.StatusCode, "]", originUrlResponse.Header)
		// if origin.OriginResourceExists && strings.HasPrefix(origin.OriginType, "s3-") {
		// 	if origin.OriginIndexETag != "" {
		// 		log.Println(i, "Origin ETag:", origin.OriginIndexETag)
		// 	}
		// 	log.Println(i, "Is Resource Exists:", origin.OriginResourceExists)
		// 	log.Println(i, "Is Website:", origin.OriginIsWebsite)
		// 	log.Println(i, "Is Bucket Public:", origin.OriginBucketPolicyIsPublic)
		// 	log.Println(i, "Origin Bucket Policy:", origin.originBucketPolicy)
		// }

	}
	route53Record := getRoute53Record(requestUrl, domainName)
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

func main() {
	r := gin.Default()
	r.GET("/explore", func(c *gin.Context) {
		requestUrl := c.Query("requestUrl")
		response := do_pipeline(requestUrl)
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Data(200, "application/json; charset=utf-8", []byte(response))
		// c.JSON(200, gin.H{
		// 	"body": response,
		// })
	})

	r.GET("/", func(c *gin.Context) {
		response := "Use the following query http://localhost:8080/explore?requestUrl=https://dev.api.sokker.info"
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Data(200, "Content-Type: text/plain; charset=utf-8", []byte(response))
	})
	r.Run(":8080") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

}
