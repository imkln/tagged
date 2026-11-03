import { useEffect, useId, useRef, useState } from 'react';
import type { Photo } from '../hooks/useLibrary';
import './PhotoViewer.css';

export default function PhotoViewer({
  photo,
  photos,
  onClose,
}: {
  photo: Photo;
  photos: Photo[];
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDivElement>(null);
  const detailsId = useId();
  const [currentId, setCurrentId] = useState(photo.id);
  const [detailsOpen, setDetailsOpen] = useState(() => window.innerWidth > 600);
  const [failedId, setFailedId] = useState<string | null>(null);
  const index = photos.findIndex((item) => item.id === currentId);
  const current = photos[index] ?? photo;

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    const background =
      document.querySelector<HTMLElement>('.app-layout > main');
    const header = document.querySelector<HTMLElement>('.app-header');
    if (background) background.inert = true;
    if (header) header.inert = true;
    document.body.style.overflow = 'hidden';
    dialog.current?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      if (background) background.inert = false;
      if (header) header.inert = false;
      opener?.focus({ preventScroll: true });
    };
  }, []);

  function navigate(offset: number) {
    const next = photos[index + offset];
    if (next) {
      setCurrentId(next.id);
      setFailedId(null);
    }
  }

  return (
    <div
      className="photo-viewer"
      role="dialog"
      aria-modal="true"
      aria-label={current.filename}
      ref={dialog}
      tabIndex={-1}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose();
        if (event.key === 'ArrowLeft') {
          event.preventDefault();
          navigate(-1);
        }
        if (event.key === 'ArrowRight') {
          event.preventDefault();
          navigate(1);
        }
        if (event.key === 'Tab') {
          const buttons = Array.from(
            dialog.current?.querySelectorAll<HTMLButtonElement>(
              'button:not(:disabled)',
            ) ?? [],
          );
          const first = buttons[0];
          const last = buttons[buttons.length - 1];
          if (
            event.shiftKey &&
            (document.activeElement === first ||
              document.activeElement === dialog.current)
          ) {
            event.preventDefault();
            last?.focus();
          } else if (
            !event.shiftKey &&
            (document.activeElement === last ||
              document.activeElement === dialog.current)
          ) {
            event.preventDefault();
            first?.focus();
          }
        }
      }}
    >
      <header className="viewer-header">
        <button type="button" onClick={onClose}>
          ← Back
        </button>
        <span>{current.filename}</span>
        <button type="button" onClick={onClose} aria-label="Close photo">
          ×
        </button>
      </header>
      <div className={`viewer-content${detailsOpen ? ' details-open' : ''}`}>
        <section className="viewer-stage" aria-label="Photo">
          {failedId === current.id ? (
            <p role="alert">Unable to load this photo.</p>
          ) : (
            <img
              key={current.id}
              src={current.originalUrl}
              alt={current.filename}
              onError={() => setFailedId(current.id)}
            />
          )}
          <nav className="viewer-navigation" aria-label="Photo navigation">
            <button
              type="button"
              aria-label="Previous photo"
              disabled={index <= 0}
              onClick={() => navigate(-1)}
            >
              ←
            </button>
            <span aria-live="polite">
              {index + 1} / {photos.length}
            </span>
            <button
              type="button"
              aria-label="Next photo"
              disabled={index < 0 || index >= photos.length - 1}
              onClick={() => navigate(1)}
            >
              →
            </button>
          </nav>
        </section>
        <div className="details-rail">
          <button
            className="details-handle"
            type="button"
            aria-label={
              detailsOpen ? 'Hide photo details' : 'Show photo details'
            }
            title={detailsOpen ? 'Hide photo details' : 'Show photo details'}
            aria-expanded={detailsOpen}
            aria-controls={detailsId}
            onClick={() => setDetailsOpen((open) => !open)}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
              aria-hidden="true"
            >
              <path d={detailsOpen ? 'm9 5 7 7-7 7' : 'm15 5-7 7 7 7'} />
            </svg>
          </button>
        </div>
        {detailsOpen && (
          <aside className="photo-details" id={detailsId}>
            <h2>Photo details</h2>
            <dl>
              <dt>Filename</dt>
              <dd>{current.filename}</dd>
              <dt>Format</dt>
              <dd>JPEG</dd>
            </dl>
          </aside>
        )}
      </div>
    </div>
  );
}
