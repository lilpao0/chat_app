package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSessionProtocolAndSigning(t *testing.T) {
	const session = "https://storage.googleapis.com/upload/storage/v1/b/test-bucket/o?upload_id=private-session"
	cancelStatus := 499
	g := &GCS{bucket: "test-bucket", signer: "media@example.iam.gserviceaccount.com"}
	g.http = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == "DELETE" {
			if req.URL.String() != session {
				t.Fatal("wrong cancellation target")
			}
			return &http.Response{StatusCode: cancelStatus, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		if req.URL.Host == "iamcredentials.googleapis.com" {
			var body map[string]string
			if json.NewDecoder(req.Body).Decode(&body) != nil || body["payload"] == "" {
				t.Fatal("missing signing payload")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"signedBlob":"c2lnbmF0dXJl"}`)), Header: make(http.Header)}, nil
		}
		if req.URL.Query().Get("ifGenerationMatch") != "0" || req.URL.Query().Get("uploadType") != "resumable" ||
			req.Header.Get("Origin") != "https://chat.example" || req.Header.Get("X-Upload-Content-Length") != "100" || req.Header.Get("X-Upload-Content-Type") != "image/jpeg" {
			t.Fatal("missing resumable preconditions or browser headers")
		}
		var metadata map[string]string
		if json.NewDecoder(req.Body).Decode(&metadata) != nil || metadata["name"] != "chat-media/test" || metadata["contentType"] != "image/jpeg" {
			t.Fatal("wrong object metadata")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Location": {session}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	got, err := g.StartUpload(context.Background(), "chat-media/test", "image/jpeg", 100, "https://chat.example")
	if err != nil || got != session {
		t.Fatalf("grant error=%v", err)
	}
	for _, cancelStatus = range []int{200, 204, 404, 410, 499} {
		if err := g.CancelUpload(context.Background(), got); err != nil {
			t.Fatalf("cancellation status %d: %v", cancelStatus, err)
		}
	}
	cancelStatus = 400
	if err := g.CancelUpload(context.Background(), got); err == nil {
		t.Fatal("failed cancellation accepted")
	}
	if err := g.CancelUpload(context.Background(), "http://127.0.0.1/private"); err == nil {
		t.Fatal("unsafe session accepted")
	}
	signed, _, err := g.SignAccess(context.Background(), "chat-media/test", 23)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatal("invalid signed URL")
	}
	lifetime, err := strconv.Atoi(u.Query().Get("X-Goog-Expires"))
	// SDK floors elapsed signing time to seconds, so a 15-minute expiry is 899-900 seconds.
	if err != nil || u.Scheme != "https" || u.Query().Get("generation") != "23" || lifetime < 899 || lifetime > 900 {
		t.Fatal("access URL is not generation-bound for 15 minutes")
	}
}

func TestImageValidation(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.White)
	for _, kind := range []string{"png", "jpeg"} {
		var data bytes.Buffer
		if kind == "png" {
			if err := png.Encode(&data, picture); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := jpeg.Encode(&data, picture, nil); err != nil {
				t.Fatal(err)
			}
		}
		if err := inspectImage(bytes.NewReader(data.Bytes()), "image/"+kind); err != nil {
			t.Fatal(err)
		}
		if err := inspectImage(bytes.NewReader(data.Bytes()[:len(data.Bytes())/2]), "image/"+kind); !errors.Is(err, entity.ErrInvalidMedia) {
			t.Fatalf("truncated image accepted: %v", err)
		}
	}
	var large bytes.Buffer
	if err := png.Encode(&large, image.NewGray(image.Rect(0, 0, 5001, 5000))); err != nil {
		t.Fatal(err)
	}
	if err := inspectImage(bytes.NewReader(large.Bytes()), "image/png"); !errors.Is(err, entity.ErrInvalidMedia) {
		t.Fatal("pixel budget ignored")
	}
}

func TestVideoTracksAndCodecValidation(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want error
	}{
		{`{"format":{"format_name":"mov,mp4,m4a"},"streams":[{"codec_type":"video","codec_name":"h264","nb_read_frames":"12"}]}`, nil},
		{`{"format":{"format_name":"mp4"},"streams":[{"codec_type":"video","codec_name":"h264","nb_read_frames":"1"},{"codec_type":"audio","codec_name":"aac"}]}`, nil},
		{`{"format":{"format_name":"mp4"},"streams":[{"codec_type":"video","codec_name":"hevc","nb_read_frames":"1"}]}`, entity.ErrUnsupportedMediaType},
		{`{"format":{"format_name":"mp4"},"streams":[{"codec_type":"video","codec_name":"h264","nb_read_frames":"0"}]}`, entity.ErrInvalidMedia},
		{`{"format":{"format_name":"mp4"},"streams":[{"codec_type":"video","codec_name":"h264","nb_read_frames":"1"},{"codec_type":"audio","codec_name":"opus"}]}`, entity.ErrUnsupportedMediaType},
		{`{"format":{"format_name":"mp4"},"streams":[{"codec_type":"video","codec_name":"h264","nb_read_frames":"1"},{"codec_type":"subtitle","codec_name":"mov_text"}]}`, entity.ErrUnsupportedMediaType},
		{`broken`, entity.ErrInvalidMedia},
	} {
		if err := checkProbe([]byte(tt.raw)); !errors.Is(err, tt.want) {
			t.Fatalf("probe error=%v want=%v", err, tt.want)
		}
	}
	secret := "https://storage.googleapis.com/?upload_id=do-not-log"
	if strings.Contains(storageError(errors.New(secret)).Error(), "do-not-log") {
		t.Fatal("storage errors leak capability URLs")
	}
}
