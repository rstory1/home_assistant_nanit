package stream

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/scgreenhalgh/home_assistant_nanit/pkg/baby"
	"github.com/scgreenhalgh/home_assistant_nanit/pkg/client"
	"github.com/scgreenhalgh/home_assistant_nanit/pkg/rtmpserver"
	"github.com/scgreenhalgh/home_assistant_nanit/pkg/utils"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/format/rtmp"
	"github.com/rs/zerolog/log"
)

// ============================================================
// ADAPTERS
// These wrap existing code to implement the stream interfaces.
// The existing code remains 100% unchanged.
// ============================================================

// --- LocalStreamerAdapter ---

// LocalStreamerAdapter wraps the existing requestLocalStreaming logic
type LocalStreamerAdapter struct {
	BabyUID      string
	TargetURL    string
	Conn         *client.WebsocketConnection
	StateManager *baby.StateManager
	RTMPServer   *rtmpserver.RTMPServer
}

// RequestLocalStream sends PUT_STREAMING request to camera
func (a *LocalStreamerAdapter) RequestLocalStream(ctx context.Context) error {
	log.Info().Str("target", a.TargetURL).Str("baby_uid", a.BabyUID).Msg("Requesting local streaming")

	awaitResponse := a.Conn.SendRequest(client.RequestType_PUT_STREAMING, &client.Request{
		Streaming: &client.Streaming{
			Id:       client.StreamIdentifier(client.StreamIdentifier_MOBILE).Enum(),
			RtmpUrl:  utils.ConstRefStr(a.TargetURL),
			Status:   client.Streaming_Status(client.Streaming_STARTED).Enum(),
			Attempts: utils.ConstRefInt32(1),
		},
	})

	_, err := awaitResponse(30 * time.Second)
	if err != nil {
		// Check for max connections error
		if strings.Contains(err.Error(), "connections above limit") {
			return ErrMaxConnections
		}
		return fmt.Errorf("local stream request failed: %w", err)
	}

	log.Info().Str("baby_uid", a.BabyUID).Msg("Local streaming successfully requested")
	a.StateManager.Update(a.BabyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_Requested))
	return nil
}

// StopLocalStream sends stop request to camera
func (a *LocalStreamerAdapter) StopLocalStream(ctx context.Context) error {
	log.Info().Str("target", a.TargetURL).Str("baby_uid", a.BabyUID).Msg("Stopping local streaming")

	a.Conn.SendRequest(client.RequestType_PUT_STREAMING, &client.Request{
		Streaming: &client.Streaming{
			Id:       client.StreamIdentifier(client.StreamIdentifier_MOBILE).Enum(),
			RtmpUrl:  utils.ConstRefStr(a.TargetURL),
			Status:   client.Streaming_Status(client.Streaming_STOPPED).Enum(),
			Attempts: utils.ConstRefInt32(1),
		},
	})

	return nil
}

// WaitForPublisher waits for the camera to connect to the RTMP server
func (a *LocalStreamerAdapter) WaitForPublisher(ctx context.Context, timeout time.Duration) error {
	if a.RTMPServer == nil {
		return fmt.Errorf("RTMPServer not configured")
	}
	err := a.RTMPServer.WaitForPublisher(a.BabyUID, timeout)
	if err != nil {
		return ErrPublisherTimeout
	}
	return nil
}

// --- BroadcasterAdapter ---

// BroadcasterFunc is a function type for broadcasting packets
type BroadcasterFunc func(pkt av.Packet)

// BroadcasterAdapter wraps a broadcast function to implement PacketReceiver
type BroadcasterAdapter struct {
	broadcast BroadcasterFunc
}

// NewBroadcasterAdapter creates a new BroadcasterAdapter
func NewBroadcasterAdapter(broadcast BroadcasterFunc) *BroadcasterAdapter {
	return &BroadcasterAdapter{broadcast: broadcast}
}

// ReceivePacket forwards packet to the broadcaster
func (a *BroadcasterAdapter) ReceivePacket(pkt av.Packet) {
	a.broadcast(pkt)
}

// --- RealRTMPClient ---

// RealRTMPClient wraps the actual RTMP library for remote connections
type RealRTMPClient struct {
	mu      sync.Mutex
	conn    *rtmp.Conn
	netConn net.Conn
	client  *rtmp.Client
}

// NewRealRTMPClient creates a new RealRTMPClient
func NewRealRTMPClient() *RealRTMPClient {
	return &RealRTMPClient{
		client: rtmp.NewClient(),
	}
}

// Connect establishes connection to RTMP server
func (c *RealRTMPClient) Connect(url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection if any
	if c.netConn != nil {
		c.netConn.Close()
		c.conn = nil
		c.netConn = nil
	}

	log.Debug().Msg("Connecting to remote RTMP stream")

	// PrepareReading = 1 (we want to read/pull the stream)
	conn, netConn, err := c.client.Dial(url, rtmp.PrepareReading)
	if err != nil {
		// Don't include URL in error - it may contain auth tokens
		return fmt.Errorf("failed to connect to remote RTMP stream: %w", err)
	}

	c.conn = conn
	c.netConn = netConn
	log.Debug().Msg("Connected to remote RTMP stream")
	return nil
}

// ReadPacket reads a packet from the RTMP stream
func (c *RealRTMPClient) ReadPacket() (av.Packet, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return av.Packet{}, fmt.Errorf("not connected")
	}

	return conn.ReadPacket()
}

// Close closes the RTMP connection
func (c *RealRTMPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.netConn != nil {
		err := c.netConn.Close()
		c.conn = nil
		c.netConn = nil
		return err
	}
	return nil
}
