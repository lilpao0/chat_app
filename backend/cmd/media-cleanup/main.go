package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/lilpao0/chat_app/backend/cmd/api/config"
	"github.com/lilpao0/chat_app/backend/internal/data/database"
	"github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/storage"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/attachment"
)

func main() {
	limit := flag.Int("limit", 10, "Maximum expired files in this run (1-100)")
	flag.Parse()
	_ = godotenv.Load()
	if err := run(*limit); err != nil {
		fmt.Fprintln(os.Stderr, "media cleanup failed:", err)
		os.Exit(1)
	}
}

func run(limit int) error {
	if limit < 1 || limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	cfg, err := config.LoadMedia()
	if err != nil {
		return err
	}
	if cfg.Bucket == "" {
		return errors.New("MEDIA_BUCKET is required")
	}
	dbCfg, err := config.LoadDB()
	if err != nil {
		return err
	}
	// One run stays below the five-minute claim lease, even when storage fails.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, dbCfg.URL)
	if err != nil {
		return errors.New("open cleanup database")
	}
	defer db.Close()
	files, err := repository.NewPostgresMediaRepository(db, cfg.SessionKey)
	if err != nil {
		return err
	}
	gcs, err := storage.NewGCS(ctx, cfg.Bucket, cfg.SignerEmail, cfg.FFProbe)
	if err != nil {
		return err
	}
	defer gcs.Close()
	done, err := attachment.NewMedia(files, gcs).Cleanup(ctx, limit)
	if encodeErr := json.NewEncoder(os.Stdout).Encode(map[string]int{"cleaned": done}); encodeErr != nil {
		return encodeErr
	}
	if err != nil {
		return errors.New("one or more cleanup operations failed; retry the job")
	}
	return nil
}
