package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	cloudstorage "cloud.google.com/go/storage"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

func mediaByteLimit(contentType string) int64 {
	if contentType == "image/jpeg" || contentType == "image/png" {
		return 10 * 1024 * 1024
	}
	if contentType == "video/mp4" {
		return 50 * 1024 * 1024
	}
	return 0
}

func (g *GCS) Inspect(ctx context.Context, key, declaredType string, declaredSize int64) (entity.VerifiedMedia, error) {
	var verified entity.VerifiedMedia
	maxBytes := mediaByteLimit(declaredType)
	if maxBytes == 0 {
		return verified, entity.ErrUnsupportedMediaType
	}
	object := g.client.Bucket(g.bucket).Object(key)
	attrs, err := object.Attrs(ctx)
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return verified, entity.ErrAttachmentNotReady
	}
	if err != nil {
		return verified, storageError(err)
	}
	if attrs.Size > maxBytes {
		return verified, entity.ErrMediaTooLarge
	}
	if attrs.Size != declaredSize || attrs.Size < 1 || attrs.ContentType != declaredType {
		return verified, entity.ErrInvalidMedia
	}
	reader, err := object.Generation(attrs.Generation).NewReader(ctx)
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return verified, entity.ErrAttachmentNotReady
	}
	if err != nil {
		return verified, storageError(err)
	}
	defer reader.Close()
	file, err := os.CreateTemp("", "chat-media-*")
	if err != nil {
		return verified, errors.New("create media inspection file")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	n, err := io.Copy(file, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return verified, storageError(err)
	}
	if n > maxBytes {
		return verified, entity.ErrMediaTooLarge
	}
	if n != declaredSize {
		return verified, entity.ErrInvalidMedia
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return verified, errors.New("read media inspection file")
	}
	var header [512]byte
	nHeader, _ := file.Read(header[:])
	actualType := http.DetectContentType(header[:nHeader])
	if actualType != declaredType {
		if mediaByteLimit(actualType) == 0 {
			return verified, entity.ErrUnsupportedMediaType
		}
		return verified, entity.ErrInvalidMedia
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return verified, errors.New("read media inspection file")
	}
	kind := "image"
	if declaredType == "video/mp4" {
		kind = "video"
		// Close before a child opens the file, including on Windows.
		if err := file.Close(); err != nil {
			return verified, errors.New("close media inspection file")
		}
		err = inspectVideo(ctx, g.ffprobe, file.Name())
	} else {
		err = inspectImage(file, declaredType)
	}
	if err != nil {
		return verified, err
	}
	return entity.VerifiedMedia{Kind: kind, ContentType: actualType, SizeBytes: n, Generation: attrs.Generation}, nil
}

func inspectImage(file io.ReadSeeker, contentType string) error {
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return entity.ErrInvalidMedia
	}
	if config.Width < 1 || config.Height < 1 || uint64(config.Width)*uint64(config.Height) > 25_000_000 {
		return entity.ErrInvalidMedia
	}
	if (format == "jpeg" && contentType != "image/jpeg") || (format == "png" && contentType != "image/png") {
		return entity.ErrInvalidMedia
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errors.New("seek media image")
	}
	decoded, actual, err := image.Decode(file)
	if err != nil || actual != format || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return entity.ErrInvalidMedia
	}
	return nil
}

type probeDocument struct {
	Format struct {
		Name string `json:"format_name"`
	} `json:"format"`
	Streams []struct {
		Type   string `json:"codec_type"`
		Codec  string `json:"codec_name"`
		Frames string `json:"nb_read_frames"`
	} `json:"streams"`
}

func checkProbe(raw []byte) error {
	var result probeDocument
	if json.Unmarshal(raw, &result) != nil || !strings.Contains(result.Format.Name, "mp4") {
		return entity.ErrInvalidMedia
	}
	videos, audio := 0, 0
	for _, s := range result.Streams {
		switch s.Type {
		case "video":
			videos++
			if s.Codec != "h264" {
				return entity.ErrUnsupportedMediaType
			}
			frames, err := strconv.ParseInt(s.Frames, 10, 64)
			if err != nil || frames <= 0 {
				return entity.ErrInvalidMedia
			}
		case "audio":
			audio++
			if s.Codec != "aac" {
				return entity.ErrUnsupportedMediaType
			}
		default:
			return entity.ErrUnsupportedMediaType
		}
	}
	if videos != 1 || audio > 1 {
		return entity.ErrUnsupportedMediaType
	}
	return nil
}

type cappedBuffer struct {
	bytes.Buffer
	max int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, errors.New("media probe output limit exceeded")
	}
	return b.Buffer.Write(p)
}

func inspectVideo(ctx context.Context, ffprobe, path string) error {
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-max_alloc", "67108864", "-protocol_whitelist", "file", "-count_frames", "-show_format", "-show_streams", "-of", "json", path)
	stdout, stderr := &cappedBuffer{max: 1024 * 1024}, &cappedBuffer{max: 64 * 1024}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return entity.ErrStorageUnavailable
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return errors.New("ffprobe executable unavailable")
	}
	if err != nil || stderr.Len() != 0 {
		return entity.ErrInvalidMedia
	}
	return checkProbe(stdout.Bytes())
}
