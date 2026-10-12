package config

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestMediaConfig(t *testing.T) {
	t.Setenv("MEDIA_BUCKET", "")
	t.Setenv("MEDIA_SESSION_KEY", "")
	if c, err := LoadMedia(); err != nil || c.Bucket != "" {
		t.Fatal("disabled media requires credentials")
	}
	t.Setenv("MEDIA_BUCKET", "private-chat-media")
	t.Setenv("MEDIA_SIGNER_EMAIL", "media@test-project.iam.gserviceaccount.com")
	t.Setenv("MEDIA_SESSION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	// LookPath checks presence; native ffprobe execution belongs to storage acceptance.
	t.Setenv("MEDIA_FFPROBE", os.Args[0])
	t.Setenv("MEDIA_UPLOADS_PER_MINUTE", "")
	if c, err := LoadMedia(); err != nil || len(c.SessionKey) != 32 || c.UploadsPerMinute != 60 {
		t.Fatal("valid media config rejected")
	}
	for _, test := range []struct{ name, value string }{
		{"MEDIA_BUCKET", "https://storage.googleapis.com/bucket"},
		{"MEDIA_SIGNER_EMAIL", "person@example.com"},
		{"MEDIA_SESSION_KEY", "invalid"},
		{"MEDIA_SESSION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16))},
		{"MEDIA_FFPROBE", "missing-chat-media-ffprobe"},
		{"MEDIA_UPLOADS_PER_MINUTE", "0"},
		{"MEDIA_UPLOADS_PER_MINUTE", "10001"},
	} {
		t.Run(test.name+test.value, func(t *testing.T) {
			t.Setenv(test.name, test.value)
			if _, err := LoadMedia(); err == nil {
				t.Fatal("invalid media config accepted")
			}
		})
	}
}
