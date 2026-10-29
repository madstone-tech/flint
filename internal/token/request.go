package token

import (
	"net/http"
	"strconv"
	"time"
)

// emptyPayloadHash is the hex SHA-256 of an empty byte slice. botocore's
// SigV4QueryAuth signs this POST with no body, which hashes to this value.
const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// newPresignRequest builds the canonical request that gets SigV4
// query-presigned: POST to the Bedrock host with the CallWithBearerToken
// action and the requested expiry encoded as X-Amz-Expires. The low-level
// v4 signer reads X-Amz-Expires off the query string, so it must be present
// before signing.
func newPresignRequest(expires time.Duration) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, endpointURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("Action", "CallWithBearerToken")
	q.Set("X-Amz-Expires", strconv.Itoa(int(expires.Seconds())))
	req.URL.RawQuery = q.Encode()
	req.Host = host
	return req, nil
}
