import { useCallback, useEffect, useRef, useState } from 'react';

export type Photo = {
  id: string;
  filename: string;
  status: string;
  error?: string;
  thumbnailUrl?: string;
  originalUrl?: string;
  capturedAt?: string;
  uploadedAt?: string;
  width?: number;
  height?: number;
  cameraMake?: string;
  cameraModel?: string;
  latitude?: number;
  longitude?: number;
};

export type Upload = {
  id: string;
  filename: string;
  status: 'queued' | 'uploading' | 'processing' | 'ready' | 'failed';
  progress: number;
  photoId?: string;
  error?: string;
};

export function useLibrary() {
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [uploads, setUploads] = useState<Upload[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [loadError, setLoadError] = useState('');
  const mounted = useRef(false);
  const requests = useRef(new Set<XMLHttpRequest>());
  const latestUploads = useRef<Upload[]>([]);
  const controller = useRef<AbortController | null>(null);

  const updateUpload = useCallback((id: string, update: Partial<Upload>) => {
    if (mounted.current) {
      setUploads((current) =>
        current.map((upload) =>
          upload.id === id ? { ...upload, ...update } : upload,
        ),
      );
    }
  }, []);

  const loadPhotos = useCallback(async () => {
    const signal = controller.current?.signal;
    try {
      const response = await fetch('/api/photos', {
        credentials: 'same-origin',
        signal,
      });
      if (!response.ok) throw new Error('Unable to load photos.');
      const body = await response.json();
      if (!Array.isArray(body.photos))
        throw new Error('Unable to load photos.');
      if (mounted.current && !signal?.aborted) {
        setPhotos(body.photos);
        setLoadError('');
      }
    } catch (cause) {
      if (mounted.current && !signal?.aborted) {
        setLoadError(
          cause instanceof Error ? cause.message : 'Unable to load photos.',
        );
      }
    } finally {
      if (mounted.current && !signal?.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    controller.current = new AbortController();
    void loadPhotos();
    return () => {
      mounted.current = false;
      controller.current?.abort();
      for (const request of requests.current) request.abort();
      requests.current.clear();
    };
  }, [loadPhotos]);

  useEffect(() => {
    latestUploads.current = uploads;
  }, [uploads]);

  const needsPolling =
    uploads.some(
      (upload) =>
        upload.status === 'processing' ||
        (upload.status === 'ready' &&
          !photos.some((photo) => photo.id === upload.photoId)),
    ) ||
    photos.some(
      (photo) => photo.status === 'processing' || photo.status === 'pending',
    );

  useEffect(() => {
    if (!needsPolling) return;
    let polling = false;
    const timer = window.setInterval(async () => {
      if (polling) return;
      polling = true;
      try {
        for (const upload of latestUploads.current.filter(
          (item) => item.status === 'processing',
        )) {
          const response = await fetch(
            `/api/uploads/${encodeURIComponent(upload.photoId!)}`,
            {
              credentials: 'same-origin',
              signal: controller.current?.signal,
            },
          );
          if (!response.ok) continue;
          const body = await response.json();
          if (body.status === 'ready' || body.status === 'failed') {
            updateUpload(upload.id, { status: body.status, error: body.error });
          }
        }
        await loadPhotos();
      } catch {
        // A temporary status-read failure must not turn a completed transfer into a failed upload.
      } finally {
        polling = false;
      }
    }, 1500);
    return () => window.clearInterval(timer);
  }, [needsPolling, loadPhotos, updateUpload]);

  async function uploadFiles(files: File[]) {
    const selected = files.map((file) => ({ file, id: crypto.randomUUID() }));
    setUploads((current) => [
      ...current,
      ...selected.map(({ file, id }): Upload => ({
        id,
        filename: file.name,
        status: 'queued',
        progress: 0,
      })),
    ]);

    for (const { file, id } of selected) {
      if (!mounted.current) break;
      if (
        file.type !== 'image/jpeg' &&
        !(file.type === '' && /\.jpe?g$/i.test(file.name))
      ) {
        const message = 'Only JPEG files are supported.';
        updateUpload(id, { status: 'failed', error: message });
        setError(message);
        continue;
      }

      try {
        updateUpload(id, { status: 'uploading' });
        const response = await fetch('/api/uploads', {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            filename: file.name,
            contentType: 'image/jpeg',
            size: file.size,
          }),
          signal: controller.current?.signal,
        });
        if (response.status === 401 || response.status === 403) {
          throw new Error('Please sign in again to upload photos.');
        }
        const authorization = await response.json();
        if (!response.ok)
          throw new Error(authorization.error || 'Unable to authorize upload.');
        updateUpload(id, { photoId: authorization.photoId });

        await new Promise<void>((resolve, reject) => {
          const request = new XMLHttpRequest();
          requests.current.add(request);
          request.open('PUT', authorization.uploadUrl);
          request.setRequestHeader('Content-Type', 'image/jpeg');
          request.setRequestHeader('If-None-Match', '*');
          request.timeout = 10 * 60 * 1000;
          request.upload.onprogress = (event) => {
            if (event.lengthComputable)
              updateUpload(id, {
                progress: Math.round((event.loaded / event.total) * 100),
              });
          };
          const finish = (failure?: string) => {
            requests.current.delete(request);
            if (failure) reject(new Error(failure));
            else resolve();
          };
          request.onload = () =>
            finish(
              request.status >= 200 && request.status < 300
                ? undefined
                : 'Upload failed.',
            );
          request.onerror = () => finish('Upload failed: connection lost.');
          request.ontimeout = () =>
            finish('Upload failed: connection timed out.');
          request.onabort = () => finish('Upload cancelled.');
          request.send(file);
        });
        updateUpload(id, { status: 'processing', progress: 100 });
        await loadPhotos();
      } catch (cause) {
        const message =
          cause instanceof Error ? cause.message : 'Upload failed.';
        updateUpload(id, { status: 'failed', error: message });
        if (mounted.current) setError(message);
      }
    }
  }

  return {
    photos,
    uploads,
    loading,
    error,
    loadError,
    uploadFiles,
    reload: loadPhotos,
  };
}
