package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// Identity holds the realm's cryptographic keypair and its realm ID.
type Identity struct {
	RealmID    string
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// Load reads an existing ECDSA private key PEM file and constructs an Identity.
func Load(realmID, keyfilePath string) (*Identity, error) {
	data, err := os.ReadFile(keyfilePath)
	if err != nil {
		return nil, fmt.Errorf("read keyfile: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM file: %s", keyfilePath)
	}

	priv, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &Identity{
		RealmID:    realmID,
		PrivateKey: priv,
		PublicKey:  &priv.PublicKey,
	}, nil
}

// Generate creates a new ECDSA keypair and writes it to the output directory.
// Produces {realmSlug}.pem (private) and {realmSlug}.pub.pem (public).
func Generate(realmID, outDir string) (*Identity, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	if err := os.MkdirAll(outDir, 0700); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// Write private key
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	privPath := filepath.Join(outDir, realmID+".pem")
	if err := writePEM(privPath, "EC PRIVATE KEY", privBytes, 0600); err != nil {
		return nil, err
	}

	// Write public key
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, err
	}
	pubPath := filepath.Join(outDir, realmID+".pub.pem")
	if err := writePEM(pubPath, "PUBLIC KEY", pubBytes, 0644); err != nil {
		return nil, err
	}

	fmt.Printf("  Private key: %s\n", privPath)
	fmt.Printf("  Public key:  %s\n", pubPath)

	return &Identity{
		RealmID:    realmID,
		PrivateKey: priv,
		PublicKey:  &priv.PublicKey,
	}, nil
}

// PublicKeyPEM returns the public key as a PEM-encoded string
// suitable for writing to the realmnet ledger.
func (id *Identity) PublicKeyPEM() (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(id.PublicKey)
	if err != nil {
		return "", err
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	return string(pem.EncodeToMemory(block)), nil
}

func writePEM(path, pemType string, data []byte, mode os.FileMode) error {
	block := &pem.Block{Type: pemType, Bytes: data}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer f.Close()
	return pem.Encode(f, block)
}
