package iam

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
