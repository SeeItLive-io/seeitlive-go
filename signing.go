package seeitlive

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// signRequest computes the HMAC request-signature headers per the
// SeeItLive wire contract:
//
//		signature = HMAC_SHA256(secret, "<timestamp>.<METHOD>.<path>.<sha256hex(rawBody)>")
//
//	  - timestamp is unix seconds.
//	  - method is upper-cased.
//	  - path is exactly the request path (no base URL, no "/api/v1" prefix).
//	  - body must be the exact bytes sent as the request body; pass nil (or an
//	    empty slice) when the request has no body, which hashes as the empty
//	    string.
//
// It returns the X-SeeItLive-Signature and X-SeeItLive-Timestamp header
// values respectively.
func signRequest(secret, method, path string, body []byte, timestamp int64) (sigHeader, tsHeader string) {
	bodyHash := sha256.Sum256(body)
	ts := strconv.FormatInt(timestamp, 10)
	message := strings.Join([]string{
		ts,
		strings.ToUpper(method),
		path,
		hex.EncodeToString(bodyHash[:]),
	}, ".")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	return "sha256=" + signature, ts
}
