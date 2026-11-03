package worker

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

func TestWorkerSavesOrientedFullImageDimensions(t *testing.T) {
	for _, orientation := range []uint16{1, 2, 3, 4, 5, 6, 7, 8} {
		t.Run(string(rune('0'+orientation)), func(t *testing.T) {
			data := orientedJPEG(t, orientation, make([]color.NRGBA, 4))

			saved := processMetadataJPEG(t, data)

			width, height := int64(80), int64(40)
			if orientation >= 5 {
				width, height = height, width
			}

			if saved["width"] != width || saved["height"] != height {
				t.Errorf("dimensions = %v by %v, want %d by %d", saved["width"], saved["height"], width, height)
			}
		})
	}

	// Dimensions describe the original, not the smaller thumbnail.
	saved := processMetadataJPEG(t, fictionalJPEG(t, 800, 400))

	if saved["width"] != int64(800) || saved["height"] != int64(400) {
		t.Errorf("full dimensions lost: %v", saved)
	}
}

func TestWorkerExtractsCameraAndCaptureDate(t *testing.T) {
	for _, offset := range []string{"", "+02:00"} {
		t.Run("offset="+offset, func(t *testing.T) {
			data := metadataJPEG(t, "2026:07:12 09:30:00", offset)

			saved := processMetadataJPEG(t, data)

			for key, want := range map[string]any{
				"capturedAt":  "2026-07-12T09:30:00" + offset,
				"cameraMake":  "Example Camera",
				"cameraModel": "Model One",
			} {
				if saved[key] != want {
					t.Errorf("%s = %v, want %v", key, saved[key], want)
				}
			}
		})
	}
}

func TestWorkerOmitsInvalidCaptureDateButKeepsOtherMetadata(t *testing.T) {
	saved := processMetadataJPEG(t, metadataJPEG(t, "2026:02:30 09:30:00", "+02:00"))

	if _, exists := saved["capturedAt"]; exists {
		t.Errorf("invalid capture date persisted: %v", saved["capturedAt"])
	}

	if saved["cameraModel"] != "Model One" {
		t.Error("invalid date discarded valid camera metadata")
	}
}

func TestWorkerAcceptsMissingAndMalformedEXIF(t *testing.T) {
	for name, data := range map[string][]byte{
		"missing":        fictionalJPEG(t, 80, 40),
		"malformed TIFF": insertMetadata(fictionalJPEG(t, 80, 40), []byte("Exif\x00\x00II*\x00\xff\xff\xff\xff")),
	} {
		t.Run(name, func(t *testing.T) {
			saved := processMetadataJPEG(t, data)

			for _, key := range []string{"capturedAt", "cameraMake", "cameraModel"} {
				if _, exists := saved[key]; exists {
					t.Errorf("unavailable %s should be omitted", key)
				}
			}
		})
	}
}

func processMetadataJPEG(t *testing.T, data []byte) map[string]any {
	t.Helper()

	fixture := newWorkerFixture()
	photo := fixture.seed("pending")
	photo.Size = int64(len(data))
	fixture.repository.records[photo.ID] = photo
	fixture.objects.data[photo.OriginalKey] = bytes.Clone(data)

	if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
		t.Fatalf("process metadata JPEG: %v", err)
	}

	saved, err := fixture.repository.Get(t.Context(), photo.ID)
	if err != nil {
		t.Fatalf("read processed photo: %v", err)
	}

	if saved.Status != "ready" {
		t.Fatalf("status = %s, want ready", saved.Status)
	}

	if !bytes.Equal(data, fixture.objects.data[photo.OriginalKey]) {
		t.Error("original bytes changed")
	}

	item, err := attributevalue.MarshalMap(saved)
	if err != nil {
		t.Fatalf("marshal saved record: %v", err)
	}

	var fields map[string]any
	if err := attributevalue.UnmarshalMap(item, &fields); err != nil {
		t.Fatalf("decode saved record: %v", err)
	}

	// DynamoDB's generic number representation is float64 by default.
	for _, key := range []string{"width", "height"} {
		if value, ok := fields[key].(float64); ok {
			fields[key] = int64(value)
		}
	}

	return fields
}

// Build fictional EXIF with independent IFDs, never real photo metadata.
func metadataJPEG(t *testing.T, date, offset string) []byte {
	t.Helper()

	type entry struct {
		tag, kind uint16
		count     uint32
		data      []byte
	}

	ascii := func(tag uint16, value string) entry {
		return entry{tag, 2, uint32(len(value) + 1), append([]byte(value), 0)}
	}

	pointer := func(tag uint16, value uint32) entry {
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, value)

		return entry{tag, 4, 1, data}
	}

	rootOffset := uint32(8)
	exifOffset := rootOffset + 2 + 3*12 + 4
	payloadOffset := exifOffset + 2 + 2*12 + 4
	tiff := make([]byte, payloadOffset)
	copy(tiff, []byte{'I', 'I', 42, 0, 8, 0, 0, 0})

	writeIFD := func(at uint32, entries []entry) {
		binary.LittleEndian.PutUint16(tiff[at:], uint16(len(entries)))
		for i, item := range entries {
			pos := int(at) + 2 + i*12
			binary.LittleEndian.PutUint16(tiff[pos:], item.tag)
			binary.LittleEndian.PutUint16(tiff[pos+2:], item.kind)
			binary.LittleEndian.PutUint32(tiff[pos+4:], item.count)
			if len(item.data) <= 4 {
				copy(tiff[pos+8:pos+12], item.data)
			} else {
				binary.LittleEndian.PutUint32(tiff[pos+8:], uint32(len(tiff)))
				tiff = append(tiff, item.data...)
			}
		}
	}

	writeIFD(rootOffset, []entry{
		ascii(0x010f, "Example Camera"),
		ascii(0x0110, "Model One"),
		pointer(0x8769, exifOffset),
	})

	capture := []entry{ascii(0x9003, date)}
	if offset != "" {
		capture = append(capture, ascii(0x9011, offset))
	}

	writeIFD(exifOffset, capture)

	return insertMetadata(fictionalJPEG(t, 80, 40), append([]byte("Exif\x00\x00"), tiff...))
}

func insertMetadata(jpeg, exif []byte) []byte {
	data := []byte{0xff, 0xd8, 0xff, 0xe1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}
	data = append(data, exif...)

	return append(data, jpeg[2:]...)
}

func TestMetadataProcessingPreservesUploadDate(t *testing.T) {
	fixture, photo, _ := pendingJPEG(t, 80, 40)

	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		t.Fatalf("marshal pending photo: %v", err)
	}

	date, err := attributevalue.Marshal("2026-08-01T12:00:00Z")
	if err != nil {
		t.Fatalf("marshal upload date: %v", err)
	}

	item["uploadedAt"] = date
	if err := attributevalue.UnmarshalMap(item, &photo); err != nil {
		t.Fatalf("decode dated photo: %v", err)
	}

	fixture.repository.records[photo.ID] = photo

	if err := fixture.worker.HandleS3Event(t.Context(), photoCreatedEvent(fixture, photo)); err != nil {
		t.Fatalf("process dated photo: %v", err)
	}

	saved, err := fixture.repository.Get(t.Context(), photo.ID)
	if err != nil {
		t.Fatalf("read dated photo: %v", err)
	}

	item, err = attributevalue.MarshalMap(saved)
	if err != nil {
		t.Fatalf("marshal processed photo: %v", err)
	}

	var fields map[string]any
	if err := attributevalue.UnmarshalMap(item, &fields); err != nil {
		t.Fatalf("decode processed photo: %v", err)
	}

	if fields["uploadedAt"] != "2026-08-01T12:00:00Z" {
		t.Errorf("upload date lost during processing: %v", fields["uploadedAt"])
	}
}
