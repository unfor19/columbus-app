package awsnetwork

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
