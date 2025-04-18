package main

import (
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

	const maxMemory = 1 << 30
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse request", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("Error closing file", err)
		}
	}(file)

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Unsupported media type", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, unableToUploadVideo, err)
		return
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			fmt.Println("Error removing temp file", err)
		}
	}(tempFile.Name())
	defer func(destinationFile *os.File) {
		err := destinationFile.Close()
		if err != nil {
			fmt.Println("Error removing temp file", err)
		}
	}(tempFile)

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, unableToUploadVideo, err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, unableToUploadVideo, err)
		return
	}

	bytes := make([]byte, 32)
	n, err := rand.Read(bytes)
	if n != len(bytes) || err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	orientation, err := getVideoOrientation(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	filename := fmt.Sprintf("%s/%s.mp4", orientation, base64.RawURLEncoding.EncodeToString(bytes))

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        tempFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, filename)
	video.VideoURL = &url
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, unableToUploadVideo, err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
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
