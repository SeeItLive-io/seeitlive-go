package seeitlive

import "testing"

const testSigningSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestSignRequestKnownAnswerVectorWithBody(t *testing.T) {
	sig, ts := signRequest(testSigningSecret, "POST", "/sessions", []byte(`{"channel":"demo"}`), 1800000000)

	wantSig := "sha256=a59401f3e80ae2e50f4b2517c8e4708de856bb93eadb1f4cfc59100db2b6e7f7"
	wantTS := "1800000000"

	if sig != wantSig {
		t.Errorf("signature header = %q, want %q", sig, wantSig)
	}
	if ts != wantTS {
		t.Errorf("timestamp header = %q, want %q", ts, wantTS)
	}
}

func TestSignRequestKnownAnswerVectorEmptyBody(t *testing.T) {
	sig, ts := signRequest(testSigningSecret, "GET", "/sessions/x/status", nil, 1800000000)

	wantSig := "sha256=3499dfb7f9581f996d537dfb108f775cb329f1f3f490f3bcb791820b4d81d5ea"
	wantTS := "1800000000"

	if sig != wantSig {
		t.Errorf("signature header = %q, want %q", sig, wantSig)
	}
	if ts != wantTS {
		t.Errorf("timestamp header = %q, want %q", ts, wantTS)
	}
}

func TestSignRequestMethodCaseInsensitive(t *testing.T) {
	upperSig, upperTS := signRequest(testSigningSecret, "POST", "/sessions", []byte(`{"channel":"demo"}`), 1800000000)
	lowerSig, lowerTS := signRequest(testSigningSecret, "post", "/sessions", []byte(`{"channel":"demo"}`), 1800000000)

	if lowerSig != upperSig {
		t.Errorf("lower-case method signature = %q, want %q (same as upper-case)", lowerSig, upperSig)
	}
	if lowerTS != upperTS {
		t.Errorf("lower-case method timestamp = %q, want %q", lowerTS, upperTS)
	}
}
