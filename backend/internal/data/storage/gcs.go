package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	cloudstorage "cloud.google.com/go/storage"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type GCS struct {
	client                  *cloudstorage.Client
	http                    *http.Client
	bucket, signer, ffprobe string
}

func NewGCS(ctx context.Context, bucket, signer, ffprobe string) (*GCS, error) {
	// The credential source belongs to the application lifetime, not one request.
	authCtx := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 10 * time.Second})
	credentials, err := google.FindDefaultCredentials(authCtx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, errors.New("Google application default credentials unavailable")
	}
	client, err := cloudstorage.NewClient(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, errors.New("initialize Google Cloud Storage")
	}
	h := oauth2.NewClient(authCtx, credentials.TokenSource)
	h.Timeout = 10 * time.Second
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &GCS{client: client, http: h, bucket: bucket, signer: signer, ffprobe: ffprobe}, nil
}

func (g *GCS) Close() error { return g.client.Close() }

func storageError(err error) error {
	if err == nil {
		return nil
	}
	var api *googleapi.Error
	if errors.As(err, &api) && api.Code != 429 && api.Code < 500 {
		return errors.New("storage operation rejected; check service configuration")
	}
	// Provider errors can contain bearer URLs. Never propagate their strings.
	return entity.ErrStorageUnavailable
}

func responseError(status int) error {
	if status == 429 || status >= 500 {
		return entity.ErrStorageUnavailable
	}
	return errors.New("storage operation rejected; check service configuration")
}

func validSession(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "storage.googleapis.com" && u.User == nil && u.Fragment == "" &&
		strings.HasPrefix(u.Path, "/upload/storage/v1/b/") && u.Query().Get("upload_id") != ""
}

func (g *GCS) StartUpload(ctx context.Context, key, contentType string, size int64, origin string) (string, error) {
	body, _ := json.Marshal(map[string]string{"name": key, "contentType": contentType})
	endpoint := "https://storage.googleapis.com/upload/storage/v1/b/" + url.PathEscape(g.bucket) + "/o?uploadType=resumable&ifGenerationMatch=0"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("construct upload session request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Upload-Content-Type", contentType)
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res, err := g.http.Do(req)
	if err != nil {
		return "", entity.ErrStorageUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return "", responseError(res.StatusCode)
	}
	session := res.Header.Get("Location")
	if !validSession(session) {
		return "", errors.New("invalid upload session response")
	}
	return session, nil
}

func (g *GCS) CancelUpload(ctx context.Context, session string) error {
	if !validSession(session) {
		return errors.New("invalid persisted upload session")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, session, nil)
	if err != nil {
		return errors.New("construct upload cancellation")
	}
	res, err := g.http.Do(req)
	if err != nil {
		return entity.ErrStorageUnavailable
	}
	defer res.Body.Close()
	// Completed sessions return 200 with object metadata; cleanup still deletes the object.
	if res.StatusCode == 200 || res.StatusCode == 499 || res.StatusCode == 404 || res.StatusCode == 410 || res.StatusCode == 204 {
		return nil
	}
	return responseError(res.StatusCode)
}

func (g *GCS) SignAccess(ctx context.Context, key string, generation int64) (string, time.Time, error) {
	expires := time.Now().UTC().Add(15 * time.Minute)
	if generation <= 0 {
		return "", expires, errors.New("missing verified object generation")
	}
	signed, err := cloudstorage.SignedURL(g.bucket, key, &cloudstorage.SignedURLOptions{
		Scheme: cloudstorage.SigningSchemeV4, GoogleAccessID: g.signer, Method: http.MethodGet, Expires: expires,
		QueryParameters: url.Values{"generation": {strconv.FormatInt(generation, 10)}},
		SignBytes: func(payload []byte) ([]byte, error) {
			body, _ := json.Marshal(map[string]string{"payload": base64.StdEncoding.EncodeToString(payload)})
			endpoint := "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + url.PathEscape(g.signer) + ":signBlob"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
			if err != nil {
				return nil, errors.New("construct signing request")
			}
			req.Header.Set("Content-Type", "application/json")
			res, err := g.http.Do(req)
			if err != nil {
				return nil, entity.ErrStorageUnavailable
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				return nil, responseError(res.StatusCode)
			}
			var result struct {
				SignedBlob string `json:"signedBlob"`
			}
			if json.NewDecoder(io.LimitReader(res.Body, 64*1024)).Decode(&result) != nil {
				return nil, errors.New("invalid signing response")
			}
			signature, err := base64.StdEncoding.DecodeString(result.SignedBlob)
			if err != nil || len(signature) == 0 {
				return nil, errors.New("invalid signing response")
			}
			return signature, nil
		},
	})
	if err != nil {
		if errors.Is(err, entity.ErrStorageUnavailable) {
			return "", expires, entity.ErrStorageUnavailable
		}
		return "", expires, errors.New("cannot sign media access URL")
	}
	return signed, expires, nil
}

func (g *GCS) DeleteObject(ctx context.Context, key string, generation int64) error {
	object := g.client.Bucket(g.bucket).Object(key)
	if generation <= 0 {
		attrs, err := object.Attrs(ctx)
		if errors.Is(err, cloudstorage.ErrObjectNotExist) {
			return nil
		}
		if err != nil {
			return storageError(err)
		}
		generation = attrs.Generation
	}
	err := object.Generation(generation).If(cloudstorage.Conditions{GenerationMatch: generation}).Delete(ctx)
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return nil
	}
	return storageError(err)
}
