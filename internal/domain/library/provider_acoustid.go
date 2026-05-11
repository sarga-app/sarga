package library

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const acoustidLookupURL = "https://api.acoustid.org/v2/lookup"

var _ IdentificationProvider = (*AcoustIDClient)(nil)

type AcoustIDClient struct {
	http    *http.Client
	limiter *rate.Limiter
	apiKey  string
}

func NewAcoustIDClient(apiKey string) *AcoustIDClient {
	return &AcoustIDClient{
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Every(time.Second/3), 3),
		apiKey:  apiKey,
	}
}

func (c *AcoustIDClient) Name() string { return "acoustid" }

// Match fingerprints the file at Metadata.path (not part of Metadata yet;
// the enricher passes the file path separately via ctx value) and looks it
// up on AcoustID.  Returns nil without error when fpcalc is unavailable or
// produces no results above the confidence threshold.
func (c *AcoustIDClient) Match(ctx context.Context, _ Metadata) (*MatchResult, error) {
	if c.apiKey == "" {
		return nil, nil
	}
	// path is injected by the enricher; gracefully absent means skip
	path, ok := ctx.Value(ctxKeyFilePath{}).(string)
	if !ok || path == "" {
		return nil, nil
	}

	dur, fp, fpErr := fingerprint(ctx, path)
	if fpErr != nil {
		return nil, nil // fpcalc absent or failed → skip silently
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	form := url.Values{
		"client":      {c.apiKey},
		"duration":    {strconv.Itoa(dur)},
		"fingerprint": {fp},
		"meta":        {"recordings releases releasegroups"},
		"format":      {"json"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, acoustidLookupURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "sarga/dev ( https://github.com/sarga-app/sarga )")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return nil, err
	}

	var result acoustidResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("acoustid: %w", err)
	}
	if result.Status != "ok" || len(result.Results) == 0 {
		return nil, nil
	}

	best := result.Results[0]
	if best.Score < 0.85 || len(best.Recordings) == 0 {
		return nil, nil
	}
	rec := best.Recordings[0]
	return &MatchResult{
		Confidence:    float64(best.Score),
		Source:        "acoustid",
		MBRecordingID: rec.ID,
	}, nil
}

// fingerprint runs fpcalc and returns (durationSec, fingerprint, error).
// Duration is an integer number of seconds as AcoustID requires.
func fingerprint(ctx context.Context, path string) (durationSec int, fpStr string, err error) {
	cmd := exec.CommandContext(ctx, "fpcalc", "-json", path) //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		return 0, "", err
	}
	var result struct {
		Duration    float64 `json:"duration"`
		Fingerprint string  `json:"fingerprint"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, "", err
	}
	return int(result.Duration), result.Fingerprint, nil
}

// ctxKeyFilePath is the context key the enricher uses to pass the file path.
type ctxKeyFilePath struct{}

// Response types ---------------------------------------------------------

type acoustidResponse struct {
	Status  string           `json:"status"`
	Results []acoustidResult `json:"results"`
}

type acoustidResult struct {
	ID         string              `json:"id"`
	Score      float32             `json:"score"`
	Recordings []acoustidRecording `json:"recordings"`
}

type acoustidRecording struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
