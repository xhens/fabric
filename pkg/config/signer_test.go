/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestSign(t *testing.T) {
	t.Parallel()

	cert, privateKey := generateCACertAndPrivateKey("org1.example.com")

	tests := []struct {
		spec        string
		privateKey  crypto.PrivateKey
		reader      io.Reader
		digest      []byte
		expectedErr string
	}{
		{
			spec:        "success",
			privateKey:  privateKey,
			reader:      rand.Reader,
			digest:      []byte("banana"),
			expectedErr: "",
		},
		{
			spec:        "unsupported rsa private key",
			privateKey:  &rsa.PrivateKey{},
			reader:      rand.Reader,
			digest:      []byte("banana"),
			expectedErr: "signing with private key of type *rsa.PrivateKey not supported",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			gt := NewGomegaWithT(t)

			signingIdentity := &SigningIdentity{
				Certificate: cert,
				PrivateKey:  tc.privateKey,
				MSPID:       "test-msp",
			}

			_, err := signingIdentity.Sign(tc.reader, tc.digest, nil)
			if tc.expectedErr == "" {
				gt.Expect(err).NotTo(HaveOccurred())
			} else {
				gt.Expect(err).To(MatchError(tc.expectedErr))
			}
		})
	}
}

func TestPublic(t *testing.T) {
	gt := NewGomegaWithT(t)

	cert, privateKey := generateCACertAndPrivateKey("org1.example.com")
	signingIdentity := &SigningIdentity{
		Certificate: cert,
		PrivateKey:  privateKey,
	}
	gt.Expect(signingIdentity.Public()).To(Equal(cert.PublicKey))
}

func TestToLowS(t *testing.T) {
	t.Parallel()

	curve := elliptic.P256()
	halfOrder := new(big.Int).Div(curve.Params().N, big.NewInt(2))

	for _, test := range []struct {
		name        string
		sig         ecdsaSignature
		expectedSig ecdsaSignature
	}{
		{
			name: "HighS",
			sig: ecdsaSignature{
				R: big.NewInt(1),
				// set S to halfOrder + 1
				S: new(big.Int).Add(halfOrder, big.NewInt(1)),
			},
			// expected signature should be (sig.R, -sig.S mod N)
			expectedSig: ecdsaSignature{
				R: big.NewInt(1),
				S: new(big.Int).Mod(new(big.Int).Neg(new(big.Int).Add(halfOrder, big.NewInt(1))), curve.Params().N),
			},
		},
		{
			name: "LowS",
			sig: ecdsaSignature{
				R: big.NewInt(1),
				// set S to halfOrder - 1
				S: new(big.Int).Sub(halfOrder, big.NewInt(1)),
			},
			// expected signature should be sig
			expectedSig: ecdsaSignature{
				R: big.NewInt(1),
				S: new(big.Int).Sub(halfOrder, big.NewInt(1)),
			},
		},
		{
			name: "HalfOrder",
			sig: ecdsaSignature{
				R: big.NewInt(1),
				// set S to halfOrder
				S: halfOrder,
			},
			// expected signature should be sig
			expectedSig: ecdsaSignature{
				R: big.NewInt(1),
				S: halfOrder,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gt := NewGomegaWithT(t)
			curve := elliptic.P256()
			key := ecdsa.PublicKey{
				Curve: curve,
			}
			gt.Expect(toLowS(key, test.sig), test.expectedSig)
		})
	}
}

// generateCACertAndPrivateKey returns CA cert and private key.
func generateCACertAndPrivateKey(orgName string) (*x509.Certificate, *ecdsa.PrivateKey) {
	serialNumber := generateSerialNumber()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "ca." + orgName,
			Organization: []string{orgName},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return generateCertAndPrivateKey(template, template, nil)
}

// generateCertAndPrivateKeyFromCACert returns a cert and private key signed by the given CACert.
func generateCertAndPrivateKeyFromCACert(orgName string, caCert *x509.Certificate, privateKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	serialNumber := generateSerialNumber()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "user." + orgName,
			Organization: []string{orgName},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	return generateCertAndPrivateKey(template, caCert, privateKey)
}

func generateCertAndPrivateKey(template, parent *x509.Certificate, parentPriv *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate private key: %s", err)
	}

	if parentPriv == nil {
		// create self-signed cert
		parentPriv = priv
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &priv.PublicKey, parentPriv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		log.Fatalf("Failed to parse certificate: %s", err)
	}

	return cert, priv
}

// generateSerialNumber returns a random serialNumber
func generateSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %s", err)
	}

	return serialNumber
}
