import { useEffect, useRef, useState } from 'react';
import type { Photo } from '../hooks/useLibrary';
import { layoutPhotos } from '../lib/photoLayout';
import './PhotoGrid.css';

export default function PhotoGrid({
  photos,
  onOpen,
}: {
  photos: Photo[];
  onOpen: (photo: Photo) => void;
}) {
  const container = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  const [ratios, setRatios] = useState<Record<string, number>>({});
  const [failedImages, setFailedImages] = useState<Set<string>>(
    () => new Set(),
  );

  useEffect(() => {
    const element = container.current;
    if (!element) return;
    setWidth(element.getBoundingClientRect().width);
    const observer = new ResizeObserver(([entry]) =>
      setWidth(entry.contentRect.width),
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const photoRatios = photos.map((photo) => ratios[photo.id] ?? 1.5);
  const rows = layoutPhotos(photoRatios, width, width < 600 ? 125 : 175, 4);
  const groups = rows.length
    ? rows
    : [{ start: 0, count: photos.length, height: 175 }];

  return (
    <div className="photo-grid" ref={container}>
      {groups.map((row) => (
        <div className="photo-row" key={row.start}>
          {photos
            .slice(row.start, row.start + row.count)
            .map((photo, offset) => (
              <div
                className="photo-card"
                role="group"
                aria-label={photo.filename}
                key={photo.id}
                style={{
                  width: row.height * photoRatios[row.start + offset],
                  height: row.height,
                }}
              >
                {photo.status === 'ready' &&
                photo.thumbnailUrl &&
                !failedImages.has(photo.id) ? (
                  <button
                    className="photo-open"
                    type="button"
                    aria-label={`Open ${photo.filename}`}
                    onClick={(event) => {
                      event.currentTarget.focus();
                      onOpen(photo);
                    }}
                  >
                    <img
                      src={photo.thumbnailUrl}
                      alt={photo.filename}
                      loading="lazy"
                      onLoad={(event) => {
                        const image = event.currentTarget;
                        if (image.naturalWidth > 0 && image.naturalHeight > 0) {
                          const ratio =
                            image.naturalWidth / image.naturalHeight;
                          setRatios((current) =>
                            current[photo.id] === ratio
                              ? current
                              : { ...current, [photo.id]: ratio },
                          );
                        }
                      }}
                      onError={() =>
                        setFailedImages(
                          (current) => new Set([...current, photo.id]),
                        )
                      }
                    />
                    <span className="photo-caption">{photo.filename}</span>
                  </button>
                ) : (
                  <div className="photo-placeholder">
                    <span>
                      {photo.status === 'failed'
                        ? 'Processing failed'
                        : failedImages.has(photo.id)
                          ? 'Preview unavailable'
                          : 'Processing…'}
                    </span>
                    <span className="placeholder-filename">
                      {photo.filename}
                    </span>
                    {photo.error && (
                      <span className="photo-error">{photo.error}</span>
                    )}
                  </div>
                )}
              </div>
            ))}
        </div>
      ))}
    </div>
  );
}
