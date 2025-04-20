package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/valivishy/tubely/internal/auth"
	"github.com/valivishy/tubely/internal/database"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
)

const unableToUploadVideo = "Unable to upload video"

func (cfg apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	err, video, unsuccessful := cfg.retrieveAndValidateVideo(w, r)
	if unsuccessful {
		return
	}

	file, mediaType, err := parseAndValidateVideoFormFile(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video upload", err)
		return
	}
	defer closer(file)

	tempFile, err := saveTempFile(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}
	defer remover(tempFile.Name())
	defer closer(tempFile)

	orientation, err := getVideoOrientation(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	filename, err := generateVideoFilename(orientation)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	url, err := cfg.uploadVideoToS3(r.Context(), tempFile, mediaType, filename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	video.VideoURL = &url
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func parseAndValidateVideoFormFile(r *http.Request) (multipart.File, string, error) {
	const maxMemory = 1 << 30
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, "", err
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		return nil, "", err
	}

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		closer(file)
		return nil, "", err
	}

	if mediaType != "video/mp4" {
		closer(file)
		return nil, "", fmt.Errorf("unsupported media type: %s", mediaType)
	}

	return file, mediaType, nil
}

func saveTempFile(src multipart.File) (*os.File, error) {
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(tempFile, src)
	if err != nil {
		closer(tempFile)
		remover(tempFile.Name())
		return nil, err
	}

	fastStartVideoFile, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		closer(tempFile)
		remover(tempFile.Name())
		return nil, err
	}

	processedFile, err := os.Open(fastStartVideoFile)
	if err != nil {
		closer(tempFile)
		remover(tempFile.Name())
		return nil, err
	}

	defer remover(tempFile.Name())
	defer remover(fastStartVideoFile)
	defer closer(tempFile)
	defer closer(processedFile)

	_, err = processedFile.Seek(0, io.SeekStart)
	if err != nil {
		closer(tempFile)
		remover(tempFile.Name())
		closer(processedFile)
		remover(processedFile.Name())
		return nil, err
	}

	return processedFile, nil
}

func generateVideoFilename(orientation string) (string, error) {
	bytes := make([]byte, 32)
	n, err := rand.Read(bytes)
	if n != len(bytes) || err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s.mp4", orientation, base64.RawURLEncoding.EncodeToString(bytes)), nil
}

func (cfg apiConfig) uploadVideoToS3(ctx context.Context, file *os.File, mediaType, filename string) (string, error) {
	_, err := cfg.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        file,
		ContentType: &mediaType,
	})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, filename)
	return url, nil
}

func (cfg apiConfig) retrieveAndValidateVideo(w http.ResponseWriter, r *http.Request) (error, database.Video, bool) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return nil, database.Video{}, true
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return nil, database.Video{}, true
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return nil, database.Video{}, true
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to find video", err)
		return nil, database.Video{}, true
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized", err)
		return nil, database.Video{}, true
	}
	return err, video, false
}
