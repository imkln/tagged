import { useState } from 'react';
import AppHeader from '../components/AppHeader';
import ScrollLoader from '../components/ScrollLoader';
import TagSearch from '../components/TagSearch';
import PhotoGrid from '../components/PhotoGrid';
import PhotoViewer from '../components/PhotoViewer';
import Notification from '../components/Notification';
import UploadProgress from '../components/UploadProgress';
import { useLibrary } from '../hooks/useLibrary';
import type { Photo } from '../hooks/useLibrary';
import './Gallery.css';

export default function Gallery({ email }: { email: string }) {
  const {
    photos,
    uploads,
    loading,
    error,
    loadError,
    uploadFiles,
    reload,
    selectedTags,
    selectTags,
    total,
    hasMore,
    loadingMore,
    pageError,
    loadMore,
  } = useLibrary();
  const [selected, setSelected] = useState<Photo | null>(null);
  const pendingUploads = uploads.filter(
    (upload) =>
      upload.status !== 'ready' &&
      !photos.some((photo) => photo.id === upload.photoId),
  );
  const viewablePhotos = photos.filter(
    (photo) => photo.status === 'ready' && photo.originalUrl,
  );

  return (
    <div className="app-layout">
      <AppHeader email={email} />
      <Notification message={selected ? '' : pageError} />
      <main className="gallery" aria-label="Photo library">
        <div
          className="gallery-toolbar"
          role="group"
          aria-label="Library actions"
        >
          <TagSearch selected={selectedTags} onChange={selectTags} />
          <span className="photo-count" aria-live="polite">
            {loading
              ? 'Loading photos…'
              : loadError && photos.length === 0
                ? 'Photos unavailable'
                : `${total} ${total === 1 ? 'photo' : 'photos'}`}
          </span>
          <label className="button upload-button">
            Upload
            <input
              className="file-input"
              aria-label="Upload photos"
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
        </div>
        {loadError && (
          <div role="alert" className="library-error">
            <span>{loadError}</span>
            <button type="button" onClick={() => void reload()}>
              Try again
            </button>
          </div>
        )}
        {error && (
          <p role="alert" className="library-error">
            {error}
          </p>
        )}
        {!loading &&
          !loadError &&
          photos.length === 0 &&
          uploads.length === 0 && (
            <section className="empty-library">
              <h1>
                {selectedTags.length ? 'No matching photos' : 'No photos yet'}
              </h1>
              {!selectedTags.length && (
                <p>Upload JPEG photos to get started.</p>
              )}
            </section>
          )}
        <UploadProgress uploads={pendingUploads} />
        <PhotoGrid photos={photos} onOpen={setSelected} />
        <ScrollLoader
          hasMore={hasMore}
          loading={loadingMore}
          error={pageError}
          onLoadMore={loadMore}
        />
      </main>
      {selected && (
        <PhotoViewer
          photos={viewablePhotos}
          photo={selected}
          total={total}
          hasMore={hasMore}
          loadingMore={loadingMore}
          pageError={pageError}
          onLoadMore={loadMore}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  );
}
