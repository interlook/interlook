package f5ltm

type getTokenPayload struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	LoginProviderName string `json:"loginProviderName"`
}

type pool struct {
	Name    string   `json:"name"`
	Monitor string   `json:"monitor"`
	Members []string `json:"members"`
}

type virtualServer struct {
	Name                     string `json:"name"`
	Destination              string `json:"destination"`
	IPProtocol               string `json:"ipProtocol"`
	Pool                     string `json:"pool"`
	SourceAddressTranslation struct {
		Type string `json:"type"`
	} `json:"sourceAddressTranslation"`
	Profiles []interface{} `json:"profiles"`
}

type tokenResponse struct {
	Username       string `json:"username"`
	LoginReference struct {
		Link string `json:"link"`
	} `json:"loginReference"`
	LoginProviderName string `json:"loginProviderName"`
	Token             struct {
		Token            string `json:"token"`
		Name             string `json:"name"`
		UserName         string `json:"userName"`
		AuthProviderName string `json:"authProviderName"`
		User             struct {
			Link string `json:"link"`
		} `json:"user"`
		Timeout          int    `json:"timeout"`
		StartTime        string `json:"startTime"`
		Address          string `json:"address"`
		Partition        string `json:"partition"`
		Generation       int    `json:"generation"`
		LastUpdateMicros int64  `json:"lastUpdateMicros"`
		ExpirationMicros int64  `json:"expirationMicros"`
		Kind             string `json:"kind"`
		SelfLink         string `json:"selfLink"`
	} `json:"token"`
	Generation       int `json:"generation"`
	LastUpdateMicros int `json:"lastUpdateMicros"`
}

// https://f5IP/mgmt/shared/authz/users/{user}
type authSelfTestResponse struct {
	Name             string `json:"name"`
	DisplayName      string `json:"displayName"`
	Shell            string `json:"shell"`
	Generation       int    `json:"generation"`
	LastUpdateMicros int    `json:"lastUpdateMicros"`
	Kind             string `json:"kind"`
	SelfLink         string `json:"selfLink"`
}

type transaction struct {
	TransID          int64  `json:"transId"`
	State            string `json:"state"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	AsyncExecution   bool   `json:"asyncExecution"`
	ValidateOnly     bool   `json:"validateOnly"`
	ExecutionTimeout int    `json:"executionTimeout"`
	ExecutionTime    int    `json:"executionTime"`
	FailureReason    string `json:"failureReason"`
	Kind             string `json:"kind"`
	SelfLink         string `json:"selfLink"`
}
