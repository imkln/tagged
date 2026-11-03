package worker

import (
	"bytes"
	"strings"
	"time"

	"github.com/bep/imagemeta"
)

func extractMetadata(data []byte, photo *photoRecord) {
	var fields imagemeta.Tags

	_, _ = imagemeta.Decode(imagemeta.Options{
		R: bytes.NewReader(data), ImageFormat: imagemeta.JPEG, Sources: imagemeta.EXIF,
		ShouldHandleTag: func(tag imagemeta.TagInfo) bool {
			if !strings.HasPrefix(tag.Namespace, "IFD0") {
				return false
			}

			switch tag.Tag {
			case "Make", "Model", "DateTimeOriginal", "OffsetTimeOriginal":
				return true
			}

			return false
		},
		HandleTag: func(tag imagemeta.TagInfo) error {
			fields.Add(tag)

			return nil
		},
	})

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
}
