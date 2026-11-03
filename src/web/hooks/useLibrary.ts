import { useCallback, useEffect, useRef, useState } from 'react';

export type Tag = { id: string; name: string; count?: number; path?: string[] };

export type Photo = {
  tags?: Tag[];
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
  const [selectedTags, setSelectedTags] = useState<Tag[]>([]);
  const filters = useRef<Tag[]>([]);
  const generation = useRef(0);
  const activeRead = useRef<symbol | null>(null);
  const loadedPages = useRef(1);
  const cursor = useRef('');
  const moreError = useRef('');
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [pageError, setPageError] = useState('');
  const [total, setTotal] = useState(0);

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
    if (activeRead.current) return;

    const token = Symbol();
    activeRead.current = token;
    const signal = controller.current?.signal;
    const requestGeneration = generation.current;
    const selected = [...filters.current];
    const current = () =>
      mounted.current &&
      !signal?.aborted &&
      requestGeneration === generation.current;

    try {
      let next = '';
      let count = 0;
      let matchedTotal = 0;
      const refreshed = new Map<string, Photo>();

      // Refresh the loaded window so upload polling never collapses a scrolled gallery.
      do {
        const body = await fetchPhotoPage(selected, next, signal);
        if (!current()) return;

        for (const photo of body.photos) refreshed.set(photo.id, photo);
        next = body.nextCursor ?? '';
        matchedTotal = body.total ?? refreshed.size;
        count += 1;
      } while (next && count < loadedPages.current);

      cursor.current = next;
      setHasMore(Boolean(next));
      setTotal(matchedTotal);
      setPhotos([...refreshed.values()]);
      setLoadError('');
    } catch (cause) {
      if (current()) {
        setLoadError(
          cause instanceof Error ? cause.message : 'Unable to load photos.',
        );
      }
    } finally {
      if (activeRead.current === token) activeRead.current = null;
      if (current()) setLoading(false);
    }
  }, []);

  const loadMore = useCallback(async (retry = false) => {
    if (activeRead.current || !cursor.current || (moreError.current && !retry))
      return;

    const token = Symbol();
    activeRead.current = token;
    const requestGeneration = generation.current;
    const signal = controller.current?.signal;
    const next = cursor.current;
    const current = () =>
      mounted.current &&
      !signal?.aborted &&
      requestGeneration === generation.current;
    setLoadingMore(true);
    moreError.current = '';
    setPageError('');

    try {
      const body = await fetchPhotoPage(filters.current, next, signal);
      if (!current()) return;

      setPhotos((previous) => {
        const merged = new Map(previous.map((photo) => [photo.id, photo]));
        for (const photo of body.photos) merged.set(photo.id, photo);

        return [...merged.values()];
      });
      loadedPages.current += 1;
      cursor.current = body.nextCursor ?? '';
      setHasMore(Boolean(cursor.current));
      if (body.total !== undefined) setTotal(body.total);
    } catch {
      if (current()) {
        moreError.current = 'Unable to load more photos.';
        setPageError(moreError.current);
      }
    } finally {
      if (activeRead.current === token) activeRead.current = null;
      if (current()) setLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    controller.current = new AbortController();
    void loadPhotos();
    return () => {
      mounted.current = false;
      controller.current?.abort();
      activeRead.current = null;
      for (const request of requests.current) request.abort();
      requests.current.clear();
    };
  }, [loadPhotos]);

  useEffect(() => {
    latestUploads.current = uploads;
  }, [uploads]);

  const needsPolling =
    uploads.some((upload) => upload.status === 'processing') ||
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

  function selectTags(tags: Tag[]) {
    filters.current = tags;
    generation.current += 1;
    activeRead.current = null;
    loadedPages.current = 1;
    cursor.current = '';
    moreError.current = '';
    setHasMore(false);
    setLoadingMore(false);
    setPageError('');
    setLoadError('');
    setSelectedTags(tags);
    setPhotos([]);
    setLoading(true);
    void loadPhotos();
  }

  return {
    total,
    hasMore,
    loadingMore,
    pageError,
    loadMore,
    selectedTags,
    selectTags,
    photos,
    uploads,
    loading,
    error,
    loadError,
    uploadFiles,
    reload: loadPhotos,
  };
}

async function fetchPhotoPage(
  tags: Tag[],
  cursor: string,
  signal?: AbortSignal,
) {
  const params = new URLSearchParams();
  for (const tag of tags) params.append('tag', tag.id);
  if (cursor) params.set('cursor', cursor);

  const path = params.size ? `/api/photos?${params}` : '/api/photos';
  const response = await fetch(path, { credentials: 'same-origin', signal });
  if (!response.ok) throw new Error('Unable to load photos.');

  const body = await response.json();
  if (!Array.isArray(body.photos)) throw new Error('Unable to load photos.');

  return body as { photos: Photo[]; total?: number; nextCursor?: string };
}
