package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getVideoOrientation(filePath string) (string, error) {
	command := exec.Command(
		"ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		filePath,
	)

	var buffer bytes.Buffer
	command.Stdout = &buffer

	if err := command.Run(); err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	var streams VideoStreams
	err := json.Unmarshal(buffer.Bytes(), &streams)
	if err != nil {
		return "", err
	}

	for _, stream := range streams.Streams {
		if stream.CodecType != "video" {
			continue
		}

		width := stream.Width
		height := stream.Height
		if width > height {
			return "landscape", nil
		} else if height > width {
			return "portrait", nil
		} else {
			return "other", nil
		}
	}

	return "", nil
}

func processVideoForFastStart(filepath string) (string, error) {
	outputFile := filepath + ".faststart"
	command := exec.Command(
		"ffmpeg",
		"-i", filepath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		outputFile,
	)

	var buffer bytes.Buffer
	command.Stdout = &buffer

	if err := command.Run(); err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	return outputFile, nil
}

type VideoStreams struct {
	Streams []Streams `json:"streams"`
}
type Disposition struct {
	Default         int `json:"default"`
	Dub             int `json:"dub"`
	Original        int `json:"original"`
	Comment         int `json:"comment"`
	Lyrics          int `json:"lyrics"`
	Karaoke         int `json:"karaoke"`
	Forced          int `json:"forced"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
	CleanEffects    int `json:"clean_effects"`
	AttachedPic     int `json:"attached_pic"`
	TimedThumbnails int `json:"timed_thumbnails"`
	NonDiegetic     int `json:"non_diegetic"`
	Captions        int `json:"captions"`
	Descriptions    int `json:"descriptions"`
	Metadata        int `json:"metadata"`
	Dependent       int `json:"dependent"`
	StillImage      int `json:"still_image"`
	Multilayer      int `json:"multilayer"`
}
type Tags struct {
	Language    string `json:"language"`
	HandlerName string `json:"handler_name"`
	VendorID    string `json:"vendor_id"`
	Encoder     string `json:"encoder"`
	Timecode    string `json:"timecode"`
}

type Streams struct {
	Index              int         `json:"index"`
	CodecName          string      `json:"codec_name,omitempty"`
	CodecLongName      string      `json:"codec_long_name,omitempty"`
	Profile            string      `json:"profile,omitempty"`
	CodecType          string      `json:"codec_type"`
	CodecTagString     string      `json:"codec_tag_string"`
	CodecTag           string      `json:"codec_tag"`
	Width              int         `json:"width,omitempty"`
	Height             int         `json:"height,omitempty"`
	CodedWidth         int         `json:"coded_width,omitempty"`
	CodedHeight        int         `json:"coded_height,omitempty"`
	ClosedCaptions     int         `json:"closed_captions,omitempty"`
	FilmGrain          int         `json:"film_grain,omitempty"`
	HasBFrames         int         `json:"has_b_frames,omitempty"`
	SampleAspectRatio  string      `json:"sample_aspect_ratio,omitempty"`
	DisplayAspectRatio string      `json:"display_aspect_ratio,omitempty"`
	PixFmt             string      `json:"pix_fmt,omitempty"`
	Level              int         `json:"level,omitempty"`
	ColorRange         string      `json:"color_range,omitempty"`
	ColorSpace         string      `json:"color_space,omitempty"`
	ColorTransfer      string      `json:"color_transfer,omitempty"`
	ColorPrimaries     string      `json:"color_primaries,omitempty"`
	ChromaLocation     string      `json:"chroma_location,omitempty"`
	FieldOrder         string      `json:"field_order,omitempty"`
	Refs               int         `json:"refs,omitempty"`
	IsAvc              string      `json:"is_avc,omitempty"`
	NalLengthSize      string      `json:"nal_length_size,omitempty"`
	ID                 string      `json:"id"`
	RFrameRate         string      `json:"r_frame_rate"`
	AvgFrameRate       string      `json:"avg_frame_rate"`
	TimeBase           string      `json:"time_base"`
	StartPts           int         `json:"start_pts"`
	StartTime          string      `json:"start_time"`
	DurationTs         int         `json:"duration_ts"`
	Duration           string      `json:"duration"`
	BitRate            string      `json:"bit_rate,omitempty"`
	BitsPerRawSample   string      `json:"bits_per_raw_sample,omitempty"`
	NbFrames           string      `json:"nb_frames"`
	ExtradataSize      int         `json:"extradata_size"`
	Disposition        Disposition `json:"disposition"`
	Tags               Tags        `json:"tags,omitempty"`
	SampleFmt          string      `json:"sample_fmt,omitempty"`
	SampleRate         string      `json:"sample_rate,omitempty"`
	Channels           int         `json:"channels,omitempty"`
	ChannelLayout      string      `json:"channel_layout,omitempty"`
	BitsPerSample      int         `json:"bits_per_sample,omitempty"`
	InitialPadding     int         `json:"initial_padding,omitempty"`
}
