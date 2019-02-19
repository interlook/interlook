package service

// Message holds config information with providers
type Message struct {
	Provider string
	Service  *Service
	Action   string // add or remove
	// FIXME: who will handle update = remove and add?
}

// Service holds the containerized service
type Service struct {
	ServiceName string
	Hosts       map[string]string
	Port        int
	TLS         bool
}
