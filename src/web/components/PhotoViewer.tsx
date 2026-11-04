import { useEffect, useId, useRef, useState } from 'react';
import type { Photo } from '../hooks/useLibrary';
import PhotoDetails from './PhotoDetails';
import Notification from './Notification';
import './PhotoViewer.css';

export default function PhotoViewer({
  photo,
  photos,
  total,
  hasMore,
  loadingMore,
  pageError,
  onLoadMore,
  onClose,
}: {
  photo: Photo;
  photos: Photo[];
  total: number;
  hasMore: boolean;
  loadingMore: boolean;
  pageError: string;
  onLoadMore: (retry?: boolean) => Promise<void>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDivElement>(null);
  const detailsId = useId();
  const [currentId, setCurrentId] = useState(photo.id);
  const [detailsOpen, setDetailsOpen] = useState(() => window.innerWidth > 600);
  const [failedId, setFailedId] = useState<string | null>(null);
  const [waitingForNext, setWaitingForNext] = useState(false);
  const [notification, setNotification] = useState('');
  const index = photos.findIndex((item) => item.id === currentId);
  const current = photos[index] ?? photo;

  useEffect(() => {
    if (!waitingForNext) return;

    if (pageError && !loadingMore) {
      setNotification(pageError);
      setWaitingForNext(false);
      return;
    }

    const next = photos[index + 1];
    if (next) {
      setCurrentId(next.id);
      setFailedId(null);
      setWaitingForNext(false);
    } else if (!loadingMore && !pageError) {
      if (hasMore) void onLoadMore();
      else setWaitingForNext(false);
    }
  }, [
    waitingForNext,
    photos,
    index,
    hasMore,
    loadingMore,
    pageError,
    onLoadMore,
  ]);

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

  useEffect(() => {
    if (!dialog.current?.contains(document.activeElement))
      dialog.current?.focus();
  }, [currentId]);

  function navigate(offset: number) {
    if (offset < 0) setWaitingForNext(false);

    const next = photos[index + offset];
    if (next) {
      setCurrentId(next.id);
      setFailedId(null);
      setWaitingForNext(false);
    } else if (offset > 0 && hasMore && !waitingForNext) {
      setNotification('');
      setWaitingForNext(true);
      if (pageError) void onLoadMore(true);
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
            dialog.current?.querySelectorAll<HTMLElement>(
              'button:not(:disabled), a[href]',
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
        <span>{current.filename}</span>
        <span>
          {total} {total === 1 ? 'photo' : 'photos'}
        </span>
        <button type="button" onClick={onClose} aria-label="Close photo">
          ×
        </button>
      </header>
      <Notification
        message={
          notification ||
          (waitingForNext && loadingMore ? 'Loading more photos…' : '')
        }
        role={notification ? 'alert' : 'status'}
      />
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
            {index > 0 ? (
              <button
                type="button"
                aria-label="Previous photo"
                onClick={() => navigate(-1)}
              >
                ←
              </button>
            ) : (
              <span />
            )}
            {index >= 0 && (index < photos.length - 1 || hasMore) ? (
              <button
                type="button"
                aria-label="Next photo"
                disabled={waitingForNext && loadingMore}
                onClick={() => navigate(1)}
              >
                →
              </button>
            ) : (
              <span />
            )}
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
            <PhotoDetails photo={current} />
          </aside>
        )}
      </div>
    </div>
  );
}
