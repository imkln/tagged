import { useState } from 'react';
import Brand from '../components/Brand';
import PhotoViewer from '../components/PhotoViewer';
import { useLibrary } from '../hooks/useLibrary';
import type { Photo } from '../hooks/useLibrary';
import './Gallery.css';

function PhotoIcon() {
  return (
    <svg viewBox="0 0 48 48" fill="none" aria-hidden="true">
      <rect x="8" y="9" width="32" height="30" rx="5" />
      <circle cx="30" cy="19" r="3" />
      <path d="m9 32 10-10 10 11 5-5 6 7" />
    </svg>
  );
}

export default function Gallery({ email }: { email: string }) {
  const { photos, uploads, loading, error, uploadFiles } = useLibrary();
  const [selected, setSelected] = useState<Photo | null>(null);
  const pendingUploads = uploads.filter(
    (upload) => !photos.some((photo) => photo.id === upload.photoId),
  );

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <Brand />
        <nav aria-label="Library">
          <a href="/gallery" className="nav-item active" aria-current="page">
            <span aria-hidden="true">▧</span> Gallery{' '}
            <span className="nav-count">{photos.length}</span>
          </a>
        </nav>
        <div className="sidebar-note">
          <span className="privacy-dot" /> Private library
          <p>Only you have access to your photos.</p>
        </div>
        <div className="account">
          <span className="avatar" aria-hidden="true">
            {email.charAt(0).toUpperCase()}
          </span>
          <span className="account-email">{email}</span>
          <form method="post" action="/auth/logout">
            <button className="sign-out" type="submit">
              Sign out
            </button>
          </form>
        </div>
      </aside>
      <main className="gallery">
        <header className="gallery-header">
          <div>
            <p className="eyebrow">Your collection</p>
            <h1>Gallery</h1>
          </div>
          <span className="private-badge">
            <span className="privacy-dot" /> Private
          </span>
        </header>
        <div className="gallery-toolbar">
          <label className="button upload-button">
            Upload photos
            <input
              className="file-input"
              type="file"
              multiple
              accept="image/jpeg,.jpg,.jpeg"
              onChange={(event) => {
                const files = Array.from(event.currentTarget.files ?? []);
                event.currentTarget.value = '';
                if (files.length) void uploadFiles(files);
              }}
            />
          </label>
          <span className="photo-count">
            {photos.length} {photos.length === 1 ? 'photo' : 'photos'}
          </span>
        </div>
        {error && (
          <p role="alert" className="library-error">
            {error}
          </p>
        )}
        {loading && <p role="status">Loading photos…</p>}
        {!loading && !error && photos.length === 0 && uploads.length === 0 && (
          <section className="empty-library" aria-labelledby="empty-title">
            <div className="empty-stack" aria-hidden="true">
              <div />
              <div />
              <div>
                <PhotoIcon />
              </div>
            </div>
            <p className="eyebrow">A fresh start</p>
            <h2 id="empty-title">Your library starts here</h2>
            <p>No photos yet. This is where your collection will live.</p>
          </section>
        )}
        {(photos.length > 0 || pendingUploads.length > 0) && (
          <div className="photo-grid">
            {photos.map((photo) => (
              <div
                className="photo-card"
                role="group"
                aria-label={photo.filename}
                key={photo.id}
              >
                {photo.status === 'ready' && photo.thumbnailUrl ? (
                  <button
                    type="button"
                    className="photo-open"
                    aria-label={`Open ${photo.filename}`}
                    onClick={(event) => {
                      event.currentTarget.focus();
                      setSelected(photo);
                    }}
                  >
                    <img
                      src={photo.thumbnailUrl}
                      alt={photo.filename}
                      loading="lazy"
                    />
                  </button>
                ) : (
                  <div className="photo-placeholder">
                    <PhotoIcon />
                    <p>
                      {photo.status === 'failed'
                        ? 'Processing failed'
                        : 'Processing'}
                    </p>
                  </div>
                )}
                <p className="photo-filename">{photo.filename}</p>
                {photo.error && <p className="photo-error">{photo.error}</p>}
              </div>
            ))}
            {pendingUploads.map((upload) => (
              <div
                className="photo-card"
                role="group"
                aria-label={upload.filename}
                key={upload.id}
              >
                <div className="photo-placeholder">
                  <PhotoIcon />
                  <p>
                    {upload.status === 'failed'
                      ? upload.error || 'Upload failed'
                      : upload.status === 'queued'
                        ? 'Waiting to upload'
                        : upload.status === 'uploading'
                          ? 'Uploading'
                          : 'Processing'}
                  </p>
                  {upload.status === 'uploading' && (
                    <progress
                      aria-label={upload.filename}
                      value={upload.progress}
                      max={100}
                    />
                  )}
                </div>
                <p className="photo-filename">{upload.filename}</p>
              </div>
            ))}
          </div>
        )}
        <footer className="gallery-footer">
          <span className="privacy-dot" /> A little space for your world.
        </footer>
      </main>
      {selected && (
        <PhotoViewer photo={selected} onClose={() => setSelected(null)} />
      )}
    </div>
  );
}
