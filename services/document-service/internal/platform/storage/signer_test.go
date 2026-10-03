package storage

import (
	"net/http"
	"testing"
	"time"
)

// Published AWS Signature Version 4 GET Object example.
// https://docs.aws.amazon.com/general/latest/gr/sigv4-signed-request-examples.html
// The credential pair is that public documentation example, not a live secret.
func TestSigV4KnownAnswerGETObject(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "examplebucket.s3.amazonaws.com"
	req.Header.Set("Host", "examplebucket.s3.amazonaws.com")
	req.Header.Set("Range", "bytes=0-9")
	now := time.Date(2013, time.May, 24, 0, 0, 0, 0, time.UTC)
	const emptyPayload = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	signRequest(req, emptyPayload, "us-east-1", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", now)

	const want = "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("signature mismatch\n got %s\nwant %s", got, want)
	}
}
