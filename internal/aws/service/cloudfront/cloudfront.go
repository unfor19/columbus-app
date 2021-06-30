package cloudfront

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/unfor19/columbus-app/internal/aws/service/iam"
	cs3 "github.com/unfor19/columbus-app/internal/aws/service/s3"
	"github.com/unfor19/columbus-app/pkg/traffic"
)

type AwsMapping struct {
	CloudFrontOrigins []CloudFrontOrigin
	TargetDomain      TargetAttributes
}

type TargetAttributes struct {
	DomainName      string
	RegisteredName  string
	TargetIpAddress string
	TargetService   string
	UrlResponse     traffic.UrlResponse
	EtagResponse    string
	Route53Record   string
	WafId           string
	NsLookup        []string
}

func ListCloudfrontDistributions(cfg aws.Config) []types.DistributionSummary {
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
	OriginType                 string
	OriginName                 string
	OriginUrl                  string
	OriginPath                 string
	OriginIndexETag            string
	originBucketPolicy         string
	OriginBucketPolicy         iam.PolicyDocument
	OriginBucketPolicyIsPublic bool
	OriginResourceExists       bool
	OriginIsWebsite            bool
	OriginUrlResponse          traffic.UrlResponse
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
			o.OriginUrlResponse.Headers = append(o.OriginUrlResponse.Headers, traffic.HttpHeader{
				Name:  name,
				Value: value,
			})
		}
	}
	o.OriginUrlResponse.StatusCode = resp.StatusCode
}

func (o *CloudFrontOrigin) setOriginPolicy(cfg aws.Config) {
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

func (o *CloudFrontOrigin) s3OriginIsPublic(cfg aws.Config) {
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

func (o *CloudFrontOrigin) setIsBucketWebsite(cfg aws.Config) {
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

func (o *CloudFrontOrigin) setIndexETag(cfg aws.Config, indexFilePath string) {
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

func GetAwsCloudfrontOrigins(cfg aws.Config, distribution types.DistributionSummary, indexFilePath string) []CloudFrontOrigin {
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
			if cs3.GetS3BucketExists(cfg, o.OriginName) {
				o.setOriginPolicy(cfg)
				o.setIndexETag(cfg, indexFilePath)
				o.s3OriginIsPublic(cfg)
				o.setIsBucketWebsite(cfg)
				o.OriginResourceExists = true
			}
			origins = append(origins, o)
		} else if origin.CustomOriginConfig != nil && strings.Contains(aws.ToString(origin.DomainName), "s3-website") {
			log.Println("Target Origin is S3 Website:", o.OriginUrl)
			o.OriginType = "s3-website"
			s3BucketName := strings.Split(o.OriginUrl, ".s3-website.")[0]
			o.OriginName = s3BucketName
			if cs3.GetS3BucketExists(cfg, o.OriginName) {
				o.setOriginPolicy(cfg)
				o.setIndexETag(cfg, indexFilePath)
				o.s3OriginIsPublic(cfg)
				o.setIsBucketWebsite(cfg)
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

func GetTargetAwsCloudfrontDistribution(cfg aws.Config, distributions []types.DistributionSummary, domainName string, indexFilePath string) (types.DistributionSummary, []CloudFrontOrigin) {
	for _, distribution := range distributions {
		// Search by origins
		origins := GetAwsCloudfrontOrigins(cfg, distribution, indexFilePath)
		for _, o := range origins {
			if strings.HasPrefix(o.OriginUrl, domainName) || strings.Contains(o.OriginUrl, ".execute-api.") {
				log.Println("Found CloudFront Distribution,", *distribution.Id, *distribution.DomainName, "by Origin", o.OriginName)
				return distribution, origins
			}
		}
	}

	return types.DistributionSummary{}, nil
}

func SetAwsCloudFrontOrigins(cfg aws.Config, targetOrigins []CloudFrontOrigin) []CloudFrontOrigin {
	for i, origin := range targetOrigins {
		log.Println(i, "Origin Type:", origin.OriginType)
		log.Println(i, "Origin Name:", origin.OriginName)
		log.Println(i, "Origin Url:", origin.OriginUrl)
		originUrlResponse := origin.getOriginUrlResponse()
		targetOrigins[i].setOriginUrlResponse(originUrlResponse)
		var bucketPolicy iam.PolicyDocument
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
	return targetOrigins
}
