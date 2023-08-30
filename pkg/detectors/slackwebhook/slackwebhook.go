package slackwebhook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/trufflesecurity/trufflehog/v3/pkg/common"
	"github.com/trufflesecurity/trufflehog/v3/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/detectorspb"
)

type Scanner struct {
	client *http.Client
}

// Ensure the Scanner satisfies the interface at compile time.
var _ detectors.Detector = (*Scanner)(nil)

var (
	// Make sure that your group is surrounded in boundary characters such as below to reduce false positives.
	keyPat = regexp.MustCompile(`(https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]{23,25})`)
)

// Keywords are used for efficiently pre-filtering chunks.
// Use identifiers in the secret preferably, or the provider name.
func (s Scanner) Keywords() []string {
	return []string{"hooks.slack.com"}
}

// FromData will find and optionally verify SlackWebhook secrets in a given set of bytes.
func (s Scanner) FromData(ctx context.Context, verify bool, data []byte) (results []detectors.Result, err error) {
	dataStr := string(data)

	matches := keyPat.FindAllStringSubmatch(dataStr, -1)

	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		resMatch := strings.TrimSpace(match[1])

		s1 := detectors.Result{
			DetectorType: detectorspb.DetectorType_SlackWebhook,
			Raw:          []byte(resMatch),
		}

		if verify {

			client := s.client
			if client == nil {
				client = common.SaneHttpClient()
			}

			payload := strings.NewReader(`{"text": ""}`)
			req, err := http.NewRequestWithContext(ctx, "POST", resMatch, payload)
			if err != nil {
				continue
			}
			req.Header.Add("Content-Type", "application/json")
			res, err := client.Do(req)
			if err == nil {
				defer res.Body.Close()
				bodyBytes, err := io.ReadAll(res.Body)
				if err != nil {
					continue
				}
				body := string(bodyBytes)

				defer res.Body.Close()
				fmt.Printf("res.StatusCode: %d\n", res.StatusCode)
				if res.StatusCode >= 200 && res.StatusCode < 300 || (res.StatusCode == 400 && (strings.Contains(body, "no_text") || strings.Contains(body, "missing_text"))) {
					s1.Verified = true
				} else if res.StatusCode == 401 || res.StatusCode == 403 {
					// The secret is determinately not verified (nothing to do)
				} else {
					s1.VerificationError = fmt.Errorf("unexpected HTTP response status %d", res.StatusCode)
				}
			} else {
				s1.VerificationError = err
			}
		}

		results = append(results, s1)
	}

	return results, nil
}

func (s Scanner) Type() detectorspb.DetectorType {
	return detectorspb.DetectorType_SlackWebhook
}
