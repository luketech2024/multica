package workbench

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestDeploymentCertificateValidatesKeyDomainAndLifetime(t *testing.T) {
	now := time.Now()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.test"}, DNSNames: []string{"example.test", "*.example.test"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	input := CertificateUpload{Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})), Domain: "www.example.test"}
	parsed, fingerprint, err := ParseDeploymentCertificate(input, now)
	if err != nil || parsed == nil || len(fingerprint) != 64 {
		t.Fatalf("valid certificate rejected: %v", err)
	}
	for _, domain := range []string{"other.test", "nested.www.example.test"} {
		input.Domain = domain
		if _, _, err := ParseDeploymentCertificate(input, now); err == nil {
			t.Fatalf("accepted wrong domain %s", domain)
		}
	}
	input.Domain = "example.test"
	if _, _, err := ParseDeploymentCertificate(input, now.Add(2*time.Hour)); err == nil {
		t.Fatal("accepted expired certificate")
	}
	if _, _, err := ParseDeploymentCertificate(input, now.Add(-2*time.Hour)); err == nil {
		t.Fatal("accepted future certificate")
	}
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	otherDER, _ := x509.MarshalPKCS8PrivateKey(other)
	input.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: otherDER}))
	if _, _, err := ParseDeploymentCertificate(input, now); err == nil {
		t.Fatal("accepted mismatched private key")
	}
}
