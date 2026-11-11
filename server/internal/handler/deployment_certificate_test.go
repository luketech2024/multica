package handler

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestDeploymentCertificateEncryptedUploadAndMetadata(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	h := *testHandler
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	h.WorkbenchSecretBox = box
	project := dbfx.Project(t, "Certificate upload")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM deployment_certificate WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
	})
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"www.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	input := workbench.CertificateUpload{Name: "Website", Domain: "www.example.test", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))}
	var metadata struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Domains []string `json:"domains"`
	}
	testutil.Call(t, h.UploadDeploymentCertificate, testutil.WithURLParams(newRequest("POST", "/api/projects/"+project+"/certificates", input), "id", project)).Want(201).JSON(&metadata)
	if metadata.Name != "Website" || len(metadata.Domains) != 1 {
		t.Fatalf("missing parsed certificate metadata: %+v", metadata)
	}
	var ciphertext, wrapped []byte
	dbfx.QueryRow(t, `SELECT secret_enc,wrapped_key FROM deployment_certificate WHERE workspace_id=$1 AND id=$2`, testWorkspaceID, metadata.ID).Scan(&ciphertext, &wrapped)
	if bytes.Contains(ciphertext, []byte("PRIVATE KEY")) {
		t.Fatal("private key stored as plaintext")
	}
	opened, err := workbench.OpenCredential(box, metadata.ID, testWorkspaceID, ciphertext, wrapped)
	if err != nil || !bytes.Contains(opened, []byte("PRIVATE KEY")) {
		t.Fatal("encrypted key could not be recovered")
	}
	clear(opened)
	var listed map[string]any
	testutil.Call(t, h.ListDeploymentCertificates, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/certificates", nil), "id", project)).Want(200).JSON(&listed)
	raw, _ := json.Marshal(listed)
	if strings.Contains(strings.ToLower(string(raw)), "private_key") {
		t.Fatal("list exposed private key")
	}
	input.Domain = "other.example.test"
	testutil.Call(t, h.UploadDeploymentCertificate, testutil.WithURLParams(newRequest("POST", "/api/projects/"+project+"/certificates", input), "id", project)).Want(400)
}
