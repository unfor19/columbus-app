package awsnetwork

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
)

func parseAwsIpRangesFile(filePath string) AwsIpRanges {
	file, _ := ioutil.ReadFile(filePath)
	data := AwsIpRanges{}
	_ = json.Unmarshal([]byte(file), &data)
	return data
}

func GetTargetAwsService(ip string, awsIpRangesFilePath string) string {
	awsIpRanges := parseAwsIpRangesFile(awsIpRangesFilePath)
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
