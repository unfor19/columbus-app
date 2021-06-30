package route53

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	cdns "github.com/unfor19/columbus-app/pkg/dns"
)

func GetRoute53Record(cfg aws.Config, requestUrl string, domainName string) string {
	registeredDomainName := cdns.GetRegisteredDomainName(requestUrl)
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
