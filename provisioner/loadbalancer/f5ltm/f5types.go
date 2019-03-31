package f5ltm

type getTokenPayload struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	LoginProviderName string `json:"loginProviderName"`
}

type pool struct {
	Name              string   `json:"name"`
	Monitor           string   `json:"monitor"`
	Description       string   `json:"description"`
	LoadBalancingMode string   `json:"loadBalancingMode"`
	Members           []string `json:"members"`
}

type poolMembers struct {
	Members []member `json:"members"`
}

type member struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type virtualServer struct {
	Name                     string `json:"name"`
	Description              string `json:"description"`
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

type virtualServerResponse struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	FullPath        string `json:"fullPath"`
	Generation      int    `json:"generation"`
	SelfLink        string `json:"selfLink"`
	AddressStatus   string `json:"addressStatus"`
	AutoLasthop     string `json:"autoLasthop"`
	CmpEnabled      string `json:"cmpEnabled"`
	ConnectionLimit int    `json:"connectionLimit"`
	Description     string `json:"description"`
	Destination     string `json:"destination"`
	Enabled         bool   `json:"enabled"`
	GtmScore        int    `json:"gtmScore"`
	IPProtocol      string `json:"ipProtocol"`
	Mask            string `json:"mask"`
	Mirror          string `json:"mirror"`
	MobileAppTunnel string `json:"mobileAppTunnel"`
	Nat64           string `json:"nat64"`
	Pool            string `json:"poolResource"`
	PoolReference   struct {
		Link string `json:"link"`
	} `json:"poolReference"`
	RateLimit                  string `json:"rateLimit"`
	RateLimitDstMask           int    `json:"rateLimitDstMask"`
	RateLimitMode              string `json:"rateLimitMode"`
	RateLimitSrcMask           int    `json:"rateLimitSrcMask"`
	ServiceDownImmediateAction string `json:"serviceDownImmediateAction"`
	Source                     string `json:"source"`
	SourceAddressTranslation   struct {
		Type string `json:"type"`
	} `json:"sourceAddressTranslation"`
	SourcePort        string `json:"sourcePort"`
	SynCookieStatus   string `json:"synCookieStatus"`
	TranslateAddress  string `json:"translateAddress"`
	TranslatePort     string `json:"translatePort"`
	VlansDisabled     bool   `json:"vlansDisabled"`
	VsIndex           int    `json:"vsIndex"`
	PoliciesReference struct {
		Link            string `json:"link"`
		IsSubcollection bool   `json:"isSubcollection"`
	} `json:"policiesReference"`
	ProfilesReference struct {
		Link            string `json:"link"`
		IsSubcollection bool   `json:"isSubcollection"`
	} `json:"profilesReference"`
}

type poolMembersResponse struct {
	Kind     string `json:"kind"`
	SelfLink string `json:"selfLink"`
	Items    []struct {
		Kind            string `json:"kind"`
		Name            string `json:"name"`
		Partition       string `json:"partition"`
		FullPath        string `json:"fullPath"`
		Generation      int    `json:"generation"`
		SelfLink        string `json:"selfLink"`
		Address         string `json:"address"`
		ConnectionLimit int    `json:"connectionLimit"`
		DynamicRatio    int    `json:"dynamicRatio"`
		Ephemeral       string `json:"ephemeral"`
		Fqdn            struct {
			Autopopulate string `json:"autopopulate"`
		} `json:"fqdn"`
		InheritProfile string `json:"inheritProfile"`
		Logging        string `json:"logging"`
		Monitor        string `json:"monitor"`
		PriorityGroup  int    `json:"priorityGroup"`
		RateLimit      string `json:"rateLimit"`
		Ratio          int    `json:"ratio"`
		Session        string `json:"session"`
		State          string `json:"state"`
	} `json:"items"`
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
