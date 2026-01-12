package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/notedit/rtmp/av"
	"github.com/rs/zerolog/log"
)

// NanitMediaHost is the Nanit cloud media server host
const NanitMediaHost = "media-secured.nanit.com"

// BuildRemoteStreamURL constructs the Nanit cloud streaming URL
// Note: The returned URL contains the auth token - never log it directly
func BuildRemoteStreamURL(babyUID, authToken string) string {
	return fmt.Sprintf("rtmps://%s/nanit/%s.%s", NanitMediaHost, babyUID, authToken)
}

// RTMPClient interface for RTMP connections (allows mocking)
type RTMPClient interface {
	Connect(url string) error
	ReadPacket() (av.Packet, error)
	Close() error
}

// PacketReceiver interface for receiving packets (usually the broadcaster)
type PacketReceiver interface {
	ReceivePacket(pkt av.Packet)
}

// TokenProvider is a function that returns a fresh auth token
// It should handle token refresh internally and return an error if auth fails completely
type TokenProvider func() (string, error)

// RemoteRelayConfig configuration for RTMPRelay
type RemoteRelayConfig struct {
	BabyUID        string
	AuthToken      string        // Initial auth token (used if TokenProvider is nil)
	TokenProvider  TokenProvider // Optional: provides fresh tokens on reconnect
	PacketReceiver PacketReceiver
	RTMPClient     RTMPClient // Optional, will create real client if nil

	// Reconnection settings
	ReconnectEnabled     bool
	ReconnectDelay       time.Duration
	MaxReconnectDelay    time.Duration
	MaxReconnectAttempts int                        // Max consecutive failures before calling OnMaxRetriesExceeded (0 = unlimited)
	OnConnected          func()                     // Called when connection succeeds
	OnMaxRetriesExceeded func(lastErr error)        // Called when max retries exceeded
	OnAuthFailed         func(err error)            // Called when auth fails and cannot be recovered (e.g., 2FA required)
}

// RTMPRelay pulls stream from Nanit cloud and pushes to local broadcaster
// Implements the RemoteRelay interface
type RTMPRelay struct {
	config RemoteRelayConfig

	mu           sync.RWMutex
	running      bool
	currentToken string // Current auth token (may be refreshed)
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewRemoteRelay creates a new RTMPRelay (implements RemoteRelay interface)
func NewRemoteRelay(config RemoteRelayConfig) *RTMPRelay {
	// Set defaults
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 5 * time.Second
	}
	if config.MaxReconnectDelay == 0 {
		config.MaxReconnectDelay = 60 * time.Second
	}

	return &RTMPRelay{
		config:       config,
		currentToken: config.AuthToken, // Initialize with provided token
	}
}

// IsRunning returns whether the relay is currently running
func (r *RTMPRelay) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// getToken returns the current token, optionally refreshing it via TokenProvider
// If refresh is true and TokenProvider is configured, it will attempt to get a fresh token
func (r *RTMPRelay) getToken(refresh bool) (string, error) {
	if refresh && r.config.TokenProvider != nil {
		token, err := r.config.TokenProvider()
		if err != nil {
			return "", err
		}
		r.mu.Lock()
		r.currentToken = token
		r.mu.Unlock()
		log.Debug().Str("baby_uid", r.config.BabyUID).Msg("Refreshed auth token for remote stream")
		return token, nil
	}

	r.mu.RLock()
	token := r.currentToken
	r.mu.RUnlock()
	return token, nil
}

// Start begins pulling from remote and relaying to local
func (r *RTMPRelay) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return errors.New("remote relay already running")
	}
	r.running = true
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.mu.Unlock()

	// Get initial token (no refresh needed yet)
	token, err := r.getToken(false)
	if err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	url := BuildRemoteStreamURL(r.config.BabyUID, token)
	log.Info().
		Str("baby_uid", r.config.BabyUID).
		Str("host", NanitMediaHost).
		Msg("Starting remote relay")

	// Try initial connection
	err = r.config.RTMPClient.Connect(url)
	if err != nil {
		if !r.config.ReconnectEnabled {
			r.mu.Lock()
			r.running = false
			r.mu.Unlock()
			return fmt.Errorf("failed to connect to remote stream: %w", err)
		}
		// If reconnect enabled, start reconnect loop in background
		log.Warn().Err(err).Str("baby_uid", r.config.BabyUID).Msg("Initial connection failed, will retry")
	} else {
		// Initial connection succeeded
		if r.config.OnConnected != nil {
			r.config.OnConnected()
		}
	}

	// Run packet relay loop in background
	r.wg.Add(1)
	go r.relayLoop()

	return nil
}

// relayLoop reads packets from remote and forwards to receiver
func (r *RTMPRelay) relayLoop() {
	defer r.wg.Done()

	reconnectDelay := r.config.ReconnectDelay
	consecutiveFailures := 0
	authFailed := false // Track if auth has completely failed

	for {
		select {
		case <-r.ctx.Done():
			r.config.RTMPClient.Close()
			return
		default:
		}

		pkt, err := r.config.RTMPClient.ReadPacket()
		if err != nil {
			log.Debug().Err(err).Str("baby_uid", r.config.BabyUID).Msg("Read packet error")

			if !r.config.ReconnectEnabled {
				r.config.RTMPClient.Close()
				return
			}

			// Attempt reconnection
			r.config.RTMPClient.Close()

			select {
			case <-r.ctx.Done():
				return
			case <-time.After(reconnectDelay):
			}

			log.Info().Str("baby_uid", r.config.BabyUID).Msg("Attempting to reconnect to remote stream")

			// Try to refresh the auth token before reconnecting
			// This handles the case where the token embedded in the RTMP URL expired
			token, tokenErr := r.getToken(true) // Request refresh
			if tokenErr != nil {
				log.Error().
					Err(tokenErr).
					Str("baby_uid", r.config.BabyUID).
					Msg("Failed to refresh auth token - authentication may have expired")

				// Notify about auth failure if not already done
				if !authFailed && r.config.OnAuthFailed != nil {
					authFailed = true
					r.config.OnAuthFailed(tokenErr)
				}

				// Continue trying with existing token in case it's a transient error
				token, _ = r.getToken(false)
			} else {
				// Token refresh succeeded, reset auth failure state
				authFailed = false
			}

			url := BuildRemoteStreamURL(r.config.BabyUID, token)
			err = r.config.RTMPClient.Connect(url)
			if err != nil {
				consecutiveFailures++
				log.Warn().
					Err(err).
					Str("baby_uid", r.config.BabyUID).
					Int("consecutive_failures", consecutiveFailures).
					Int("max_attempts", r.config.MaxReconnectAttempts).
					Msg("Reconnection failed")

				// Check if max retries exceeded
				if r.config.MaxReconnectAttempts > 0 && consecutiveFailures >= r.config.MaxReconnectAttempts {
					log.Error().
						Str("baby_uid", r.config.BabyUID).
						Int("consecutive_failures", consecutiveFailures).
						Msg("Max reconnection attempts exceeded, notifying pool")

					if r.config.OnMaxRetriesExceeded != nil {
						r.config.OnMaxRetriesExceeded(err)
					}

					// Reset counter and continue trying (but pool can now try local)
					consecutiveFailures = 0
				}

				// Exponential backoff
				reconnectDelay = time.Duration(float64(reconnectDelay) * 2)
				if reconnectDelay > r.config.MaxReconnectDelay {
					reconnectDelay = r.config.MaxReconnectDelay
				}
				continue
			}

			// Reset on successful connection
			consecutiveFailures = 0
			reconnectDelay = r.config.ReconnectDelay
			authFailed = false
			log.Info().Str("baby_uid", r.config.BabyUID).Msg("Reconnected to remote stream")

			// Notify that we're connected
			if r.config.OnConnected != nil {
				r.config.OnConnected()
			}
			continue
		}

		// Forward packet to receiver
		r.config.PacketReceiver.ReceivePacket(pkt)
	}
}

// Stop stops the relay
func (r *RTMPRelay) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()

	// Close the RTMP client to unblock any pending ReadPacket() calls
	// This must happen before wg.Wait() to avoid deadlock
	r.config.RTMPClient.Close()

	// Wait for relay loop to finish
	r.wg.Wait()

	log.Info().Str("baby_uid", r.config.BabyUID).Msg("Remote relay stopped")
	return nil
}
