package mqtt

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/scgreenhalgh/home_assistant_nanit/pkg/baby"
	"github.com/scgreenhalgh/home_assistant_nanit/pkg/utils"
	"github.com/rs/zerolog/log"
)

// ErrBabyNotAuthorized indicates a command was received for an unauthorized baby
var ErrBabyNotAuthorized = errors.New("baby UID not authorized")

type SendLightCommandHandler func(nightLightState bool)
type SendStandbyCommandHandler func(standbyState bool)

// Connection - MQTT context
type Connection struct {
	Opts                      Opts
	StateManager              *baby.StateManager
	client                    MQTT.Client
	sendLightCommandHandler   SendLightCommandHandler
	sendStandbyCommandHandler SendStandbyCommandHandler

	// Discovery support
	discoveryPublisher *DiscoveryPublisher
	registeredBabies   map[string]string // babyUID -> babyName
	babiesMutex        sync.RWMutex
	discoveryPublished map[string]bool // Track which babies have had discovery published
}

// NewConnection - constructor
func NewConnection(opts Opts) *Connection {
	// Initialize MQTT client options
	mqttOpts := MQTT.NewClientOptions()
	mqttOpts.AddBroker(opts.BrokerURL)
	mqttOpts.SetClientID(opts.TopicPrefix)
	mqttOpts.SetUsername(opts.Username)
	mqttOpts.SetPassword(opts.Password)
	mqttOpts.SetCleanSession(false)

	return &Connection{
		Opts:               opts,
		client:             MQTT.NewClient(mqttOpts),
		registeredBabies:   make(map[string]string),
		discoveryPublished: make(map[string]bool),
	}
}

// RegisterBaby registers a baby for MQTT discovery
// Should be called after babies are fetched from the API
func (conn *Connection) RegisterBaby(babyUID, babyName string) {
	conn.babiesMutex.Lock()
	conn.registeredBabies[babyUID] = babyName
	conn.babiesMutex.Unlock()

	// If we're already connected and have a discovery publisher, publish immediately
	if conn.discoveryPublisher != nil {
		conn.publishDiscoveryForBaby(babyUID, babyName)
	}
}

// RegisterBabies registers multiple babies for MQTT discovery
func (conn *Connection) RegisterBabies(babies []baby.Baby) {
	for _, b := range babies {
		conn.RegisterBaby(b.UID, b.Name)
	}
}

// IsAuthorizedBaby checks if a baby UID is registered and authorized for commands
func (conn *Connection) IsAuthorizedBaby(babyUID string) bool {
	if babyUID == "" {
		return false
	}
	conn.babiesMutex.RLock()
	_, exists := conn.registeredBabies[babyUID]
	conn.babiesMutex.RUnlock()
	return exists
}

// ValidateBabyCommandAuth validates that a baby UID is both valid and authorized
// Returns nil if the baby is valid and authorized, otherwise returns an error
func (conn *Connection) ValidateBabyCommandAuth(babyUID string) error {
	if babyUID == "" {
		return errors.New("baby UID cannot be empty")
	}

	// Validate format first
	if err := baby.ValidateBabyUID(babyUID); err != nil {
		return fmt.Errorf("invalid baby UID format: %w", err)
	}

	// Check authorization
	if !conn.IsAuthorizedBaby(babyUID) {
		return ErrBabyNotAuthorized
	}

	return nil
}

func (conn *Connection) publishDiscoveryForBaby(babyUID, babyName string) {
	conn.babiesMutex.Lock()
	alreadyPublished := conn.discoveryPublished[babyUID]
	conn.babiesMutex.Unlock()

	if alreadyPublished {
		return
	}

	if err := conn.discoveryPublisher.PublishDiscovery(babyUID, babyName); err != nil {
		log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to publish discovery config")
		return
	}

	conn.babiesMutex.Lock()
	conn.discoveryPublished[babyUID] = true
	conn.babiesMutex.Unlock()
}

func (conn *Connection) publishAllDiscovery() {
	conn.babiesMutex.RLock()
	babies := make(map[string]string)
	for uid, name := range conn.registeredBabies {
		babies[uid] = name
	}
	conn.babiesMutex.RUnlock()

	for babyUID, babyName := range babies {
		conn.publishDiscoveryForBaby(babyUID, babyName)
	}
}

// Run - runs the mqtt connection handler
func (conn *Connection) Run(manager *baby.StateManager, ctx utils.GracefulContext) {
	conn.StateManager = manager

	utils.RunWithPerseverance(func(attempt utils.AttemptContext) {
		runMqtt(conn, attempt)
	}, ctx, utils.PerseverenceOpts{
		RunnerID:       "mqtt",
		ResetThreshold: 2 * time.Second,
		Cooldown: []time.Duration{
			2 * time.Second,
			10 * time.Second,
			1 * time.Minute,
		},
	})
}

func (conn *Connection) RegisterLightHandler(sendLightCommandHandler SendLightCommandHandler) {
	conn.sendLightCommandHandler = sendLightCommandHandler
}

func (conn *Connection) subscribeToLightCommand() {
	commandTopic := fmt.Sprintf("%v/babies/+/night_light/switch", conn.Opts.TopicPrefix)
	log.Debug().
		Str("topic", commandTopic).
		Msg("Subscribing to command topic")

	lightMessageHandler := func(mqttConn MQTT.Client, msg MQTT.Message) {
		// Extract baby UID and command from topic
		// Expected format: {prefix}/babies/{babyUID}/night_light/switch
		parts := strings.Split(msg.Topic(), "/")
		if len(parts) < 5 {
			log.Error().Str("topic", msg.Topic()).Msg("Invalid command topic format")
			return
		}

		babyUID := parts[2]
		command := parts[4]

		// Validate baby UID format and authorization
		if err := conn.ValidateBabyCommandAuth(babyUID); err != nil {
			log.Error().Err(err).Str("baby_uid", babyUID).Msg("Unauthorized MQTT command rejected")
			return
		}

		// Handle different commands
		switch command {
		case "switch":
			enabled := string(msg.Payload()) == "true"
			log.Debug().
				Str("baby", babyUID).
				Bool("enabled", enabled).
				Str("payload", string(msg.Payload())).
				Msg("Received light command")

			if conn.sendLightCommandHandler != nil {
				conn.sendLightCommandHandler(enabled)
			}
		default:
			log.Warn().Str("command", command).Msg("Unknown command received")
		}
	}

	if token := conn.client.Subscribe(commandTopic, 0, lightMessageHandler); token.Wait() && token.Error() != nil {
		log.Error().Err(token.Error()).Str("topic", commandTopic).Msg("Failed to subscribe to command topic")
	}
}

func (conn *Connection) RegisterStandyHandler(sendStandbyCommandHandler SendStandbyCommandHandler) {
	conn.sendStandbyCommandHandler = sendStandbyCommandHandler
}

// GetTopicPrefix returns the MQTT topic prefix
func (conn *Connection) GetTopicPrefix() string {
	return conn.Opts.TopicPrefix
}

// GetClient returns the underlying MQTT client
func (conn *Connection) GetClient() MQTT.Client {
	return conn.client
}

func (conn *Connection) subscribeToStandbyCommand() {
	commandTopic := fmt.Sprintf("%v/babies/+/standby/switch", conn.Opts.TopicPrefix)
	log.Debug().
		Str("topic", commandTopic).
		Msg("Subscribing to command topic")

	standbyMessageHandler := func(mqttConn MQTT.Client, msg MQTT.Message) {
		// Extract baby UID and command from topic
		// Expected format: {prefix}/babies/{babyUID}/standby/switch
		parts := strings.Split(msg.Topic(), "/")
		if len(parts) < 5 {
			log.Error().Str("topic", msg.Topic()).Msg("Invalid command topic format")
			return
		}

		babyUID := parts[2]
		command := parts[4]

		// Validate baby UID format and authorization
		if err := conn.ValidateBabyCommandAuth(babyUID); err != nil {
			log.Error().Err(err).Str("baby_uid", babyUID).Msg("Unauthorized MQTT command rejected")
			return
		}

		// Handle different commands
		switch command {
		case "switch":
			enabled := string(msg.Payload()) == "true"
			log.Debug().
				Str("baby", babyUID).
				Bool("enabled", enabled).
				Str("payload", string(msg.Payload())).
				Msg("Received standby command")

			if conn.sendStandbyCommandHandler != nil {
				conn.sendStandbyCommandHandler(enabled)
			}
		default:
			log.Warn().Str("command", command).Msg("Unknown command received")
		}
	}

	if token := conn.client.Subscribe(commandTopic, 0, standbyMessageHandler); token.Wait() && token.Error() != nil {
		log.Error().Err(token.Error()).Str("topic", commandTopic).Msg("Failed to subscribe to command topic")
	}
}

func runMqtt(conn *Connection, attempt utils.AttemptContext) {

	if token := conn.client.Connect(); token.Wait() && token.Error() != nil {
		log.Error().Str("broker_url", conn.Opts.BrokerURL).Err(token.Error()).Msg("Unable to connect to MQTT broker")
		attempt.Fail(token.Error())
		return
	}

	log.Info().Str("broker_url", conn.Opts.BrokerURL).Msg("Successfully connected to MQTT broker")

	// Initialize discovery publisher if enabled
	if conn.Opts.DiscoveryEnabled {
		conn.discoveryPublisher = NewDiscoveryPublisher(DiscoveryConfig{
			Enabled:     true,
			TopicPrefix: conn.Opts.TopicPrefix,
			NodeID:      "nanit",
			RTMPAddr:    conn.Opts.RTMPAddr,
		}, conn.client)

		// Publish discovery for all registered babies
		conn.publishAllDiscovery()
	}

	unsubscribe := conn.StateManager.Subscribe(func(babyUID string, state baby.State) {
		publish := func(key string, value interface{}, retained bool) {
			topic := fmt.Sprintf("%v/babies/%v/%v", conn.Opts.TopicPrefix, babyUID, key)
			log.Trace().Str("topic", topic).Interface("value", value).Bool("retained", retained).Msg("MQTT publish")

			token := conn.client.Publish(topic, 0, retained, fmt.Sprintf("%v", value))
			if token.Wait(); token.Error() != nil {
				log.Error().Err(token.Error()).Msgf("Unable to publish %v update", key)
			}
		}

		for key, value := range state.AsMap(false) {
			publish(key, value, false)
		}

		if state.StreamState != nil && *state.StreamState != baby.StreamState_Unknown {
			publish("is_stream_alive", *state.StreamState == baby.StreamState_Alive, true)
		}

		if state.StreamType != nil && *state.StreamType != baby.StreamType_None {
			publish("stream_source", state.StreamType.String(), true)
		}
	})

	// Subscribe to accept light mqtt messages
	conn.subscribeToLightCommand()
	conn.subscribeToStandbyCommand()

	// Wait until interrupt signal is received
	<-attempt.Done()

	log.Debug().Msg("Closing MQTT connection on interrupt")
	unsubscribe()
	conn.client.Disconnect(250)
}
