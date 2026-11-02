import { useEffect, useRef, useState } from 'react';
import type { Photo } from '../hooks/useLibrary';
import './PhotoViewer.css';

export default function PhotoViewer({
  photo,
  onClose,
}: {
  photo: Photo;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    dialog.current?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      opener?.focus();
    };
  }, []);

  return (
    <div
      className="photo-viewer"
      role="dialog"
      aria-modal="true"
      aria-label={photo.filename}
      ref={dialog}
      tabIndex={-1}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose();
        if (event.key === 'Tab') {
          event.preventDefault();
          dialog.current?.querySelector<HTMLButtonElement>('button')?.focus();
        }
      }}
    >
      <header>
        <span>{photo.filename}</span>
        <button type="button" onClick={onClose} aria-label="Close photo">
          Close
        </button>
      </header>
      {failed ? (
        <p role="alert">Unable to load this photo.</p>
      ) : (
        <img
          src={photo.originalUrl}
          alt={photo.filename}
          onError={() => setFailed(true)}
        />
      )}
    </div>
  );
}
