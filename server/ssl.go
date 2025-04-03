package server

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Generate a self-signed X.509 certificate for a TLS server. Outputs to
// 'cert.pem' and 'key.pem' and will overwrite existing files.

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/utils"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}

// EnableSSL do some check when ssl enabled
func EnableSSL(options libs.Options) error {
	if options.Server.DisableSSL {
		return nil
	}

	if utils.FileExists(options.Server.CertFile) && utils.FileExists(options.Server.KeyFile) {
		return nil
	}

	ok := GenerateSSL(options.Server.CertFile, options.Server.KeyFile)
	if ok {
		return nil
	}
	utils.ErrorF("error create ssl key at: %v", options.Server.CertFile)
	return fmt.Errorf("error create SSL Key")
}

// GenerateSSL generate SSL key
func GenerateSSL(certFile string, keyFile string) bool {
	host := "localhost"              // "Comma-separated hostnames and IPs to generate a certificate for
	validFrom := ""                  // "Creation date formatted as Jan 1 15:04:05 2011
	validFor := 365 * 24 * time.Hour // "Duration that certificate is valid for
	isCA := false                    // "whether this cert should be its own Certificate Authority
	rsaBits := 4096                  // "Size of RSA key to generate. Ignored if --ecdsa-curve is set
	ecdsaCurve := "P256"             // "ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521
	ed25519Key := false              // "Generate an Ed25519 key"

	var priv interface{}
	var err error
	switch ecdsaCurve {
	case "":
		if ed25519Key {
			_, priv, err = ed25519.GenerateKey(rand.Reader)
		} else {
			priv, err = rsa.GenerateKey(rand.Reader, rsaBits)
		}
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		log.Fatalf("Unrecognized elliptic curve: %q", ecdsaCurve)
	}
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	var notBefore time.Time
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			log.Fatalf("Failed to parse creation date: %v", err)
		}
	}

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Not Localhost"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		utils.ErrorF("Failed to create certificate: %v", err)
		return false
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		utils.ErrorF("Failed to open %s for writing: %v", certFile, err)
		return false
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		utils.ErrorF("Failed to write data to cert.pem: %v", err)
		return false
	}
	if err := certOut.Close(); err != nil {
		utils.ErrorF("Error closing cert.pem: %v", err)
		return false
	}
	//log.Print("wrote cert.pem\n")

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		utils.ErrorF("Failed to open %sfor writing: %v", keyFile, err)
		return false
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		utils.ErrorF("Unable to marshal private key: %v", err)
		return false
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		utils.ErrorF("Failed to write data to key.pem: %v", err)
		return false
	}
	if err := keyOut.Close(); err != nil {
		utils.ErrorF("Error closing key.pem: %v", err)
		return false
	}
	//log.Print("wrote key.pem\n")
	return true
}
