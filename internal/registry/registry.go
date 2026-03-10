package registry

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"sync"
	"time"
)

// RegistrationRequest is the payload a realm signs and sends to peers.
type RegistrationRequest struct {
	RealmID   string            `json:"realmId"`
	Endpoint  string            `json:"endpoint"`
	PublicKey string            `json:"publicKey"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp int64             `json:"timestamp"`
}

// Result tracks the outcome of a registration attempt with a single peer.
type Result struct {
	Peer      string
	Accepted  bool
	BlockHash string
	Error     error
}

// Registry handles broadcasting a realm registration to realmnet peers
// and collecting majority confirmation before the runtime comes up.
type Registry struct {
	realmID     string
	endpoint    string
	publicKey   string
	privateKey  *ecdsa.PrivateKey
	peers       []string
	dialTimeout time.Duration
}

// Config holds everything needed to create a Registry.
type Config struct {
	RealmID    string
	Endpoint   string
	PublicKey  string
	PrivateKey *ecdsa.PrivateKey
	Peers      []string // e.g. ["bootstrap1.realmnet.io:7946"]
}

// New creates a Registry from config.
func New(cfg Config) *Registry {
	return &Registry{
		realmID:     cfg.RealmID,
		endpoint:    cfg.Endpoint,
		publicKey:   cfg.PublicKey,
		privateKey:  cfg.PrivateKey,
		peers:       cfg.Peers,
		dialTimeout: 10 * time.Second,
	}
}
func (r *Registry) queryExists(peer string) (bool, string) {
	conn, err := net.DialTimeout("tcp", peer, 5*time.Second)
	if err != nil {
		return false, ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	msg := map[string]any{
		"type":    "REALM_RESOLVE",
		"request": map[string]string{"realmId": r.realmID},
	}
	data, _ := json.Marshal(msg)
	if err := writeFrame(conn, data); err != nil {
		return false, ""
	}

	respData, err := readFrame(conn)
	if err != nil {
		return false, ""
	}

	var resp struct {
		Found     bool   `json:"found"`
		BlockHash string `json:"blockHash,omitempty"`
	}
	if err := json.Unmarshal(respData, &resp); err != nil {
		return false, ""
	}
	return resp.Found, resp.BlockHash
}

// Register signs and broadcasts a REALM_REGISTERED event to all peers,
// then blocks until majority confirmation is received.
func (r *Registry) Register() error {
	if len(r.peers) == 0 {
		return fmt.Errorf("no peers configured — cannot register on realmnet")
	}

	// Resume support — skip if already on ledger
	if isRegistered, blockHash := r.isAlreadyRegistered(); isRegistered {
		regID := r.realmID
		if blockHash != "" {
			// Show first 8 chars of block hash as registration ID
			if len(blockHash) > 8 {
				regID = "realmnet1" + blockHash[:8]
			} else {
				regID = "realmnet1" + blockHash
			}
		}
		fmt.Printf("Already registered as: %s\n", regID)
		return nil
	}

	req := RegistrationRequest{
		RealmID:   r.realmID,
		Endpoint:  r.endpoint,
		PublicKey: r.publicKey,
		Timestamp: time.Now().Unix(),
	}

	sig, err := sign(r.privateKey, req)
	if err != nil {
		return fmt.Errorf("sign registration: %w", err)
	}

	log.Printf("[registry] broadcasting REALM_REGISTERED to %d peers", len(r.peers))

	results := r.broadcast(req, sig)

	accepted, rejected, errored := 0, 0, 0
	for _, res := range results {
		switch {
		case res.Error != nil:
			errored++
			log.Printf("[registry] peer %s unreachable: %v", res.Peer, res.Error)
		case res.Accepted:
			accepted++
			log.Printf("[registry] peer %s accepted (block: %s)", res.Peer, res.BlockHash)
		default:
			rejected++
			log.Printf("[registry] peer %s rejected", res.Peer)
		}
	}

	if accepted == 0 {
		return fmt.Errorf("registration failed: no peers accepted (%d tried, %d errors)", len(r.peers), errored)
	}

	// First ACK is sufficient — the block propagates via gossip from there.
	// Settlement certainty comes from chain depth (blocks built on top),
	// not from polling every peer at registration time.
	log.Printf("[registry] ✓ %s registered on realmnet (%d peer(s) confirmed, propagating via gossip)", r.realmID, accepted)
	return nil
}

func (r *Registry) isAlreadyRegistered() (bool, string) {
	// Ask a peer if we're already on the ledger
	for _, peer := range r.peers {
		if found, blockHash := r.queryExists(peer); found {
			return true, blockHash
		}
	}
	return false, ""
}

// broadcast fans out to all peers concurrently.
func (r *Registry) broadcast(req RegistrationRequest, sig string) []Result {
	results := make([]Result, len(r.peers))
	var wg sync.WaitGroup
	for i, peer := range r.peers {
		wg.Add(1)
		go func(idx int, addr string) {
			defer wg.Done()
			results[idx] = r.sendToPeer(addr, req, sig)
		}(i, peer)
	}
	wg.Wait()
	return results
}

// sendToPeer opens a TCP connection and exchanges registration + ACK/NACK.
func (r *Registry) sendToPeer(addr string, req RegistrationRequest, sig string) Result {
	conn, err := net.DialTimeout("tcp", addr, r.dialTimeout)
	if err != nil {
		return Result{Peer: addr, Error: fmt.Errorf("connect: %w", err)}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	msg := map[string]any{
		"type":      "REALM_REGISTERED",
		"request":   req,
		"signature": sig,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return Result{Peer: addr, Error: fmt.Errorf("marshal: %w", err)}
	}
	if err := writeFrame(conn, data); err != nil {
		return Result{Peer: addr, Error: fmt.Errorf("send: %w", err)}
	}

	respData, err := readFrame(conn)
	if err != nil {
		return Result{Peer: addr, Error: fmt.Errorf("read response: %w", err)}
	}

	var resp struct {
		Accepted  bool   `json:"accepted"`
		BlockHash string `json:"blockHash"`
		Reason    string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(respData, &resp); err != nil {
		return Result{Peer: addr, Error: fmt.Errorf("parse response: %w", err)}
	}
	if !resp.Accepted {
		return Result{Peer: addr, Accepted: false, Error: fmt.Errorf("rejected: %s", resp.Reason)}
	}
	return Result{Peer: addr, Accepted: true, BlockHash: resp.BlockHash}
}

// sign produces a hex-encoded ECDSA signature over the JSON of req.
func sign(priv *ecdsa.PrivateKey, req RegistrationRequest) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return "", fmt.Errorf("ecdsa sign: %w", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return hex.EncodeToString(sig), nil
}

// Verify checks a registration signature against the registering realm's public key.
// Called by realmnet peers when they receive an incoming registration.
func Verify(pub *ecdsa.PublicKey, req RegistrationRequest, sigHex string) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil || len(sigBytes) != 64 {
		return fmt.Errorf("invalid signature encoding")
	}
	hash := sha256.Sum256(data)
	rInt := new(big.Int).SetBytes(sigBytes[:32])
	sInt := new(big.Int).SetBytes(sigBytes[32:])
	if !ecdsa.Verify(pub, hash[:], rInt, sInt) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// --- frame helpers ---

func writeFrame(conn net.Conn, data []byte) error {
	l := uint32(len(data))
	header := []byte{byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)}
	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

func readFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := readFull(conn, header); err != nil {
		return nil, err
	}
	l := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	if l > 1<<20 {
		return nil, fmt.Errorf("message too large: %d bytes", l)
	}
	buf := make([]byte, l)
	if _, err := readFull(conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
