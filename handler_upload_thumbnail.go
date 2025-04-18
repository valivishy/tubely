package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/valivishy/tubely/internal/auth"
)

const couldNotSaveThumbnail = "Could not save thumbnail"

func (cfg apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse request", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Unsupported media type", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized", err)
		return
	}

	bytes := make([]byte, 32)
	n, err := rand.Read(bytes)
	if n != len(bytes) || err != nil {
		respondWithError(w, http.StatusInternalServerError, couldNotSaveThumbnail, err)
		return
	}

	filename := fmt.Sprintf(
		"%s.%s",
		base64.RawURLEncoding.EncodeToString(bytes),
		strings.Split(header.Filename, ".")[1],
	)

	destination := filepath.Join(cfg.assetsRoot, filename)
	destinationFile, err := os.Create(destination)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, couldNotSaveThumbnail, err)
		return
	}

	if _, err = io.Copy(destinationFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, couldNotSaveThumbnail, err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)
	video.ThumbnailURL = &url
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, couldNotSaveThumbnail, err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
