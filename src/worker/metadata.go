package worker

import (
	"bytes"
	"math"
	"strings"
	"time"

	"github.com/bep/imagemeta"
	"github.com/imkln/tagged/src/tags"
)

func extractMetadata(data []byte, photo *photoRecord) Place {
	embedded := Place{}
	var fields imagemeta.Tags

	_, _ = imagemeta.Decode(imagemeta.Options{
		R: bytes.NewReader(data), ImageFormat: imagemeta.JPEG, Sources: imagemeta.EXIF | imagemeta.XMP | imagemeta.IPTC,
		ShouldHandleTag: func(tag imagemeta.TagInfo) bool {
			if tag.Source != imagemeta.EXIF {
				return tag.Tag == "Country" || tag.Tag == "City"
			}

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
			fields.Add(tag)

			return nil
		},
	})

	// Metadata is optional: retain valid fields decoded before a malformed field.
	for _, name := range []string{"Country", "City"} {
		for _, source := range []map[string]imagemeta.TagInfo{fields.XMP(), fields.IPTC()} {
			if value, ok := source[name].Value.(string); ok && strings.TrimSpace(value) != "" {
				if name == "Country" {
					embedded.Country = value
				} else {
					embedded.City = value
				}
				break
			}
		}
	}

	applyPlaceTags(photo, embedded)
	exif := fields.EXIF()
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
		return embedded
	}

	if math.IsNaN(latitude) || math.IsNaN(longitude) || latitude < 0 || latitude > 90 || longitude < 0 || longitude > 180 {
		return embedded
	}

	if northSouth == "S" {
		latitude = -latitude
	}

	if eastWest == "W" {
		longitude = -longitude
	}

	photo.Latitude, photo.Longitude = &latitude, &longitude

	return embedded
}

func applyPlaceTags(photo *photoRecord, place Place) {
	paths := [][]string{{place.Country}}
	if strings.TrimSpace(place.City) != "" && tags.Normalize(place.City) != tags.Normalize(place.Country) {
		paths = append(paths, []string{place.Country, place.City})
	}

	photo.Tags = tags.ReplaceSourcePaths(photo.Tags, paths, "metadata")
}
