package mqtt

// Opts - holds configuration needed to establish connection to the broker
type Opts struct {
	BrokerURL string
	ClientID  string

	Username string
	Password string

	TopicPrefix string

	// Discovery options for Home Assistant MQTT auto-discovery
	DiscoveryEnabled bool
	RTMPAddr         string // RTMP server address for stream URL sensor
}
