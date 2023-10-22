package internal

type Configuration struct {
	MemberID        string `envconfig:"MEMBER_ID" required:"true"`
	ElectionGroup   string `envconfig:"ELECTION_GROUP" required:"true"`
	PodName         string `envconfig:"POD_NAME" required:"true"`
	Namespace       string `envconfig:"NAMESPACE" required:"true"`
	LeaseDuration   int    `envconfig:"LEASE_DURATION" default:"15"`
	RenewalDeadline int    `envconfig:"RENEWAL_DEADLINE" default:"10"`
	RetryPeriod     int    `envconfig:"RETRY_PERIOD" default:"5"`
}
