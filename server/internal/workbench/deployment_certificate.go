package workbench

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"time"
)

type CertificateUpload struct {
	Name        string `json:"name"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
	Domain      string `json:"domain"`
}

func ParseDeploymentCertificate(input CertificateUpload, now time.Time) (*x509.Certificate, string, error) {
	pair, err := tls.X509KeyPair([]byte(input.Certificate), []byte(input.PrivateKey))
	if err != nil {
		return nil, "", errors.New("certificate PEM and private key do not form a valid matching pair")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, "", errors.New("certificate could not be parsed")
	}
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, "", errors.New("certificate is expired or not yet valid")
	}
	if len(leaf.DNSNames) == 0 && len(leaf.IPAddresses) == 0 {
		return nil, "", errors.New("certificate has no subject alternative names")
	}
	if input.Domain != "" && leaf.VerifyHostname(input.Domain) != nil {
		return nil, "", errors.New("certificate does not cover the application domain")
	}
	previous := leaf
	for _, der := range pair.Certificate[1:] {
		issuer, parseErr := x509.ParseCertificate(der)
		if parseErr != nil || previous.CheckSignatureFrom(issuer) != nil || now.Before(issuer.NotBefore) || !now.Before(issuer.NotAfter) {
			return nil, "", errors.New("certificate chain is invalid or expired")
		}
		previous = issuer
	}
	fingerprint := sha256.Sum256(leaf.Raw)
	return leaf, hex.EncodeToString(fingerprint[:]), nil
}
