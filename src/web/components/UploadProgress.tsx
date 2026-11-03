import type { Upload } from '../hooks/useLibrary';
import './UploadProgress.css';

export default function UploadProgress({ uploads }: { uploads: Upload[] }) {
  if (!uploads.length) return null;
  return (
    <section className="upload-progress" aria-label="Uploads">
      {uploads.map((upload) => (
        <div
          className="upload-item"
          role="group"
          aria-label={upload.filename}
          key={upload.id}
        >
          <span className="upload-filename">{upload.filename}</span>
          <span
            className={
              upload.status === 'failed' ? 'photo-error' : 'upload-status'
            }
          >
            {upload.status === 'failed'
              ? upload.error || 'Upload failed'
              : upload.status === 'queued'
                ? 'Waiting to upload'
                : upload.status === 'uploading'
                  ? 'Uploading'
                  : 'Processing…'}
          </span>
          {upload.status === 'uploading' && (
            <progress
              aria-label={upload.filename}
              value={upload.progress}
              max={100}
            />
          )}
        </div>
      ))}
    </section>
  );
}
