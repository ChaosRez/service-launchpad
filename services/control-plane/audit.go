package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultGCSUploadEndpoint = "https://storage.googleapis.com"
	defaultAuditPrefix       = "control-plane/deployments"
	metadataTokenURL         = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
)

type auditResult struct {
	Bucket string `json:"bucket,omitempty"`
	Object string `json:"object,omitempty"`
}

type deploymentAuditRecord struct {
	ID                   string            `json:"id"`
	RecordedAt           time.Time         `json:"recordedAt"`
	Service              serviceDefinition `json:"service"`
	Namespace            string            `json:"namespace"`
	Status               string            `json:"status"`
	DurationMilliseconds int64             `json:"durationMilliseconds"`
	Error                string            `json:"error,omitempty"`
	Result               applyResult       `json:"result,omitempty"`
	Manifests            manifestBundle    `json:"manifests"`
}

type auditRecorder interface {
	RecordDeployment(ctx context.Context, record deploymentAuditRecord) (auditResult, error)
}

type gcsAuditRecorder struct {
	bucket     string
	prefix     string
	endpoint   string
	token      string
	httpClient *http.Client
	now        func() time.Time
}

func newGCSAuditRecorder(bucket, prefix, endpoint, token string) auditRecorder {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = defaultAuditPrefix
	}
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		endpoint = defaultGCSUploadEndpoint
	}

	return &gcsAuditRecorder{
		bucket:     bucket,
		prefix:     prefix,
		endpoint:   endpoint,
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		now:        time.Now,
	}
}

func (r *gcsAuditRecorder) RecordDeployment(ctx context.Context, record deploymentAuditRecord) (auditResult, error) {
	if record.RecordedAt.IsZero() {
		record.RecordedAt = r.now().UTC()
	}
	if record.ID == "" {
		record.ID = buildAuditRecordID(record)
	}

	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return auditResult{}, fmt.Errorf("marshal audit record: %w", err)
	}

	objectName := r.objectName(record)
	uploadURL, err := url.Parse(r.endpoint + "/upload/storage/v1/b/" + url.PathEscape(r.bucket) + "/o")
	if err != nil {
		return auditResult{}, fmt.Errorf("build GCS upload URL: %w", err)
	}
	query := uploadURL.Query()
	query.Set("uploadType", "media")
	query.Set("name", objectName)
	uploadURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL.String(), bytes.NewReader(body))
	if err != nil {
		return auditResult{}, fmt.Errorf("build GCS upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	token := r.token
	if token == "" {
		token, err = fetchMetadataAccessToken(ctx, r.httpClient)
		if err != nil {
			return auditResult{}, err
		}
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return auditResult{}, fmt.Errorf("upload audit record to GCS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return auditResult{}, fmt.Errorf("upload audit record to GCS returned %s", resp.Status)
	}

	return auditResult{
		Bucket: r.bucket,
		Object: objectName,
	}, nil
}

func (r *gcsAuditRecorder) objectName(record deploymentAuditRecord) string {
	recordedAt := record.RecordedAt.UTC()
	serviceName := sanitizeAuditPathSegment(record.Service.Name)
	if serviceName == "" {
		serviceName = "unknown-service"
	}

	filename := fmt.Sprintf("%s-%s.json", recordedAt.Format("20060102T150405.000000000Z"), sanitizeAuditPathSegment(record.Status))
	return path.Join(
		r.prefix,
		recordedAt.Format("2006"),
		recordedAt.Format("01"),
		recordedAt.Format("02"),
		serviceName,
		filename,
	)
}

func buildAuditRecordID(record deploymentAuditRecord) string {
	timestamp := record.RecordedAt.UTC().Format("20060102T150405.000000000Z")
	serviceName := sanitizeAuditPathSegment(record.Service.Name)
	if serviceName == "" {
		serviceName = "unknown-service"
	}
	status := sanitizeAuditPathSegment(record.Status)
	if status == "" {
		status = "unknown"
	}
	return fmt.Sprintf("%s/%s/%s", timestamp, serviceName, status)
}

func sanitizeAuditPathSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func fetchMetadataAccessToken(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataTokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("build metadata token request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch metadata access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch metadata access token returned %s", resp.Status)
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode metadata access token: %w", err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("metadata access token response did not include access_token")
	}
	return payload.AccessToken, nil
}
