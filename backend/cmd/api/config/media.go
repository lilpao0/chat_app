package config

import (
	"encoding/base64"
	"errors"
	"net/mail"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type MediaConfig struct {
	Bucket, SignerEmail, FFProbe string
	SessionKey                   []byte
	UploadsPerMinute             int
}

func LoadMedia() (MediaConfig, error) {
	c := MediaConfig{Bucket: os.Getenv("MEDIA_BUCKET"), SignerEmail: os.Getenv("MEDIA_SIGNER_EMAIL"), FFProbe: os.Getenv("MEDIA_FFPROBE"), UploadsPerMinute: 60}
	if c.Bucket == "" {
		return c, nil
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]$`).MatchString(c.Bucket) {
		return c, errors.New("invalid MEDIA_BUCKET")
	}
	address, err := mail.ParseAddress(c.SignerEmail)
	if err != nil || address.Address != c.SignerEmail || address.Name != "" || !strings.HasSuffix(c.SignerEmail, ".gserviceaccount.com") {
		return c, errors.New("MEDIA_SIGNER_EMAIL must be a service account email")
	}
	c.SessionKey, err = base64.StdEncoding.DecodeString(os.Getenv("MEDIA_SESSION_KEY"))
	if err != nil || len(c.SessionKey) != 32 {
		return c, errors.New("MEDIA_SESSION_KEY must be base64 for 32 bytes")
	}
	if c.FFProbe == "" {
		c.FFProbe = "ffprobe"
	}
	if _, err := exec.LookPath(c.FFProbe); err != nil {
		return c, errors.New("MEDIA_FFPROBE executable unavailable")
	}
	if raw := os.Getenv("MEDIA_UPLOADS_PER_MINUTE"); raw != "" {
		c.UploadsPerMinute, err = strconv.Atoi(raw)
		if err != nil || c.UploadsPerMinute < 1 || c.UploadsPerMinute > 10000 {
			return c, errors.New("MEDIA_UPLOADS_PER_MINUTE must be between 1 and 10000")
		}
	}
	return c, nil
}
