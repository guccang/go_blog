package obsstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	obs "github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

var ErrDisabled = errors.New("obs store is disabled")

type Config struct {
	Endpoint         string `json:"endpoint,omitempty"`
	Bucket           string `json:"bucket,omitempty"`
	AccessKey        string `json:"ak,omitempty"`
	SecretKey        string `json:"sk,omitempty"`
	Region           string `json:"region,omitempty"`
	KeyPrefix        string `json:"key_prefix,omitempty"`
	PathStyle        bool   `json:"path_style,omitempty"`
	DisableSSLVerify bool   `json:"disable_ssl_verify,omitempty"`
}

type PutObjectRequest struct {
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
	Metadata    map[string]string
}

type SignedURL struct {
	URL       string
	Method    string
	ExpiresAt int64
	Headers   map[string]string
}

type Store struct {
	cfg    Config
	client *obs.ObsClient
}

func New(cfg Config) (*Store, error) {
	cfg = cfg.normalized()
	if cfg.isZero() {
		return &Store{cfg: cfg}, nil
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	client, err := obs.New(
		cfg.AccessKey,
		cfg.SecretKey,
		cfg.Endpoint,
		obs.WithPathStyle(cfg.PathStyle),
		obs.WithSslVerify(!cfg.DisableSSLVerify),
		obs.WithRegion(cfg.Region),
		obs.WithConnectTimeout(10),
		obs.WithSocketTimeout(30),
	)
	if err != nil {
		return nil, fmt.Errorf("create obs client: %w", err)
	}
	return &Store{cfg: cfg, client: client}, nil
}

func (s *Store) Enabled() bool {
	return s != nil && s.client != nil && strings.TrimSpace(s.cfg.Bucket) != ""
}

func (s *Store) Bucket() string {
	if s == nil {
		return ""
	}
	return s.cfg.Bucket
}

func (s *Store) NormalizeKey(key string) string {
	return joinKey(s.cfg.KeyPrefix, key)
}

func (s *Store) PutObject(_ context.Context, req PutObjectRequest) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	key := s.NormalizeKey(req.Key)
	if key == "" {
		return fmt.Errorf("object key is required")
	}
	if req.Body == nil {
		return fmt.Errorf("object body is required")
	}
	_, err := s.client.PutObject(&obs.PutObjectInput{
		PutObjectBasicInput: obs.PutObjectBasicInput{
			ObjectOperationInput: obs.ObjectOperationInput{
				Bucket:   s.cfg.Bucket,
				Key:      key,
				Metadata: cloneStringMap(req.Metadata),
			},
			HttpHeader: obs.HttpHeader{
				ContentType: strings.TrimSpace(req.ContentType),
			},
			ContentLength: req.Size,
		},
		Body: req.Body,
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *Store) HeadObject(_ context.Context, key string) (bool, error) {
	if !s.Enabled() {
		return false, ErrDisabled
	}
	key = s.NormalizeKey(key)
	if key == "" {
		return false, fmt.Errorf("object key is required")
	}
	_, err := s.client.HeadObject(&obs.HeadObjectInput{
		Bucket: s.cfg.Bucket,
		Key:    key,
	})
	if err == nil {
		return true, nil
	}
	if IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("head object %s: %w", key, err)
}

func (s *Store) CreateSignedGetURL(_ context.Context, key string, ttl time.Duration) (*SignedURL, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	key = s.NormalizeKey(key)
	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("invalid signed url ttl: %s", ttl)
	}
	output, err := s.client.CreateSignedUrl(&obs.CreateSignedUrlInput{
		Method:  obs.HttpMethodGet,
		Bucket:  s.cfg.Bucket,
		Key:     key,
		Expires: int(ttl.Seconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("create signed url %s: %w", key, err)
	}
	headers := make(map[string]string, len(output.ActualSignedRequestHeaders))
	for header, values := range output.ActualSignedRequestHeaders {
		if len(values) > 0 {
			headers[header] = values[0]
		}
	}
	return &SignedURL{
		URL:       output.SignedUrl,
		Method:    "GET",
		ExpiresAt: time.Now().Add(ttl).UnixMilli(),
		Headers:   headers,
	}, nil
}

func IsNotFound(err error) bool {
	var obsErr obs.ObsError
	if errors.As(err, &obsErr) {
		return obsErr.StatusCode == 404 || strings.EqualFold(obsErr.Code, "NoSuchKey")
	}
	return false
}

func (c Config) normalized() Config {
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.Bucket = strings.TrimSpace(c.Bucket)
	c.AccessKey = strings.TrimSpace(c.AccessKey)
	c.SecretKey = strings.TrimSpace(c.SecretKey)
	c.Region = strings.TrimSpace(c.Region)
	c.KeyPrefix = strings.Trim(c.KeyPrefix, "/")
	return c
}

func (c Config) isZero() bool {
	return c.Endpoint == "" &&
		c.Bucket == "" &&
		c.AccessKey == "" &&
		c.SecretKey == "" &&
		c.Region == "" &&
		c.KeyPrefix == ""
}

func (c Config) validate() error {
	missing := make([]string, 0, 4)
	if c.Endpoint == "" {
		missing = append(missing, "endpoint")
	}
	if c.Bucket == "" {
		missing = append(missing, "bucket")
	}
	if c.AccessKey == "" {
		missing = append(missing, "ak")
	}
	if c.SecretKey == "" {
		missing = append(missing, "sk")
	}
	if len(missing) > 0 {
		return fmt.Errorf("obs config missing fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

func joinKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, "/")
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
