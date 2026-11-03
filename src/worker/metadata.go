package worker

import (
	"bytes"
	"math"
	"strings"
	"time"

	"github.com/bep/imagemeta"
)

func extractMetadata(data []byte, photo *photoRecord) {
	var tags imagemeta.Tags

	_, _ = imagemeta.Decode(imagemeta.Options{
		R: bytes.NewReader(data), ImageFormat: imagemeta.JPEG, Sources: imagemeta.EXIF,
		ShouldHandleTag: func(tag imagemeta.TagInfo) bool {
			if !strings.HasPrefix(tag.Namespace, "IFD0") {
				return false
			}

			switch tag.Tag {
			case "Make", "Model", "DateTimeOriginal", "OffsetTimeOriginal", "GPSLatitude", "GPSLatitudeRef", "GPSLongitude", "GPSLongitudeRef":
				return true
			}

			return false
		},
		HandleTag: func(tag imagemeta.TagInfo) error {
			tags.Add(tag)

			return nil
		},
	})

	// Metadata is optional: retain valid fields decoded before a malformed field.
	exif := tags.EXIF()
	text := func(name string) string {
		value, _ := exif[name].Value.(string)
		return strings.TrimSpace(strings.Trim(value, "\x00"))
	}

	photo.CameraMake = text("Make")
	photo.CameraModel = text("Model")

	if captured, err := time.Parse("2006:01:02 15:04:05", text("DateTimeOriginal")); err == nil {
		photo.CapturedAt = captured.Format("2006-01-02T15:04:05")

		if offset := text("OffsetTimeOriginal"); offset != "" {
			if _, err := time.Parse("-07:00", offset); err == nil {
				photo.CapturedAt += offset
			}
		}
	}

	latitude, hasLatitude := exif["GPSLatitude"].Value.(float64)
	longitude, hasLongitude := exif["GPSLongitude"].Value.(float64)
	northSouth, eastWest := text("GPSLatitudeRef"), text("GPSLongitudeRef")
	if !hasLatitude || !hasLongitude || (northSouth != "N" && northSouth != "S") || (eastWest != "E" && eastWest != "W") {
		return
	}

	if math.IsNaN(latitude) || math.IsNaN(longitude) || latitude < 0 || latitude > 90 || longitude < 0 || longitude > 180 {
		return
	}

	if northSouth == "S" {
		latitude = -latitude
	}

	if eastWest == "W" {
		longitude = -longitude
	}

	photo.Latitude, photo.Longitude = &latitude, &longitude
}
