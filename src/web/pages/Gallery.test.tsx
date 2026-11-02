import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import Gallery from './Gallery';

const readyPhoto = {
  id: 'photo-ready',
  filename: 'garden.jpg',
  status: 'ready',
  thumbnailUrl: '/api/photos/photo-ready/thumbnail',
  originalUrl: '/api/photos/photo-ready/original',
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function mockLibrary(photos: unknown[] = []) {
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    if (path === '/api/photos' && (!init?.method || init.method === 'GET')) {
      return jsonResponse({ photos, total: photos.length });
    }
    throw new Error(`Unexpected request: ${init?.method ?? 'GET'} ${path}`);
  });
  vi.stubGlobal('fetch', fetch);
  return fetch;
}

function mockUploads() {
  const requests: XMLHttpRequest[] = [];
  const urls: string[] = [];
  vi.spyOn(XMLHttpRequest.prototype, 'open').mockImplementation(function (
    this: XMLHttpRequest,
    _method: string,
    url: string | URL,
  ) {
    urls.push(String(url));
  });
  vi.spyOn(XMLHttpRequest.prototype, 'setRequestHeader').mockImplementation(
    () => {},
  );
  const send = vi
    .spyOn(XMLHttpRequest.prototype, 'send')
    .mockImplementation(function (this: XMLHttpRequest) {
      requests.push(this);
    });
  return { requests, urls, send };
}

function finishUpload(request: XMLHttpRequest, status: number) {
  Object.defineProperty(request, 'status', {
    configurable: true,
    value: status,
  });
  Object.defineProperty(request, 'readyState', {
    configurable: true,
    value: 4,
  });
  request.dispatchEvent(new Event('load'));
  request.dispatchEvent(new Event('loadend'));
}

function showGallery() {
  return render(<Gallery email="owner@example.test" />);
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

it('renders saved photo thumbnails', async () => {
  mockLibrary([readyPhoto]);
  showGallery();

  expect(
    await screen.findByRole('img', { name: 'garden.jpg' }),
  ).toHaveAttribute('src', readyPhoto.thumbnailUrl);
  expect(
    screen.queryByText('Your library starts here'),
  ).not.toBeInTheDocument();
});

it('loads the original only when opening the viewer', async () => {
  mockLibrary([readyPhoto]);
  showGallery();

  const thumbnail = await screen.findByRole('img', { name: 'garden.jpg' });
  expect(
    document.querySelector(`img[src="${readyPhoto.originalUrl}"]`),
  ).toBeNull();
  const openPhoto = screen.getByRole('button', { name: /open garden.jpg/i });
  expect(openPhoto).toContainElement(thumbnail);
  fireEvent.click(openPhoto);

  const viewer = await screen.findByRole('dialog', { name: 'garden.jpg' });
  expect(
    within(viewer).getByRole('img', { name: 'garden.jpg' }),
  ).toHaveAttribute('src', readyPhoto.originalUrl);
});

it('closes the viewer with Escape and restores focus', async () => {
  mockLibrary([readyPhoto]);
  showGallery();
  const openPhoto = await screen.findByRole('button', {
    name: /open garden.jpg/i,
  });
  fireEvent.click(openPhoto);
  const viewer = await screen.findByRole('dialog', { name: 'garden.jpg' });

  fireEvent.keyDown(viewer, { key: 'Escape' });
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  expect(openPhoto).toHaveFocus();
});

it('provides an explicit close button in the photo viewer', async () => {
  mockLibrary([readyPhoto]);
  showGallery();
  fireEvent.click(
    await screen.findByRole('button', { name: /open garden.jpg/i }),
  );

  const viewer = await screen.findByRole('dialog', { name: 'garden.jpg' });
  fireEvent.click(within(viewer).getByRole('button', { name: /close/i }));
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
});

it('shows a thumbnail failure without a broken image', async () => {
  mockLibrary([
    { id: 'photo-failed', filename: 'bridge.jpg', status: 'failed' },
  ]);
  showGallery();

  const failed = await screen.findByRole('group', { name: 'bridge.jpg' });
  expect(within(failed).getByText(/failed/i)).toBeInTheDocument();
  expect(screen.queryByRole('img')).not.toBeInTheDocument();
});

it('reports a library load failure instead of displaying an empty library', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(jsonResponse({ error: 'Unavailable' }, 503)),
  );
  showGallery();

  expect(await screen.findByRole('alert')).toHaveTextContent(/unable to load/i);
  expect(
    screen.queryByText('Your library starts here'),
  ).not.toBeInTheDocument();
});

it('rejects unsupported files without requesting upload authorization', async () => {
  const fetch = mockLibrary();
  showGallery();
  const input = await screen.findByLabelText(/upload photos/i);

  fireEvent.change(input, {
    target: {
      files: [
        new File(['fictional image'], 'drawing.png', { type: 'image/png' }),
      ],
    },
  });

  expect(await screen.findByRole('alert')).toHaveTextContent(/JPEG/i);
  expect(fetch.mock.calls.some(([url]) => String(url) === '/api/uploads')).toBe(
    false,
  );
});

function mockBatch() {
  const transport = mockUploads();
  const states = new Map<string, string>();
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);

    if (path === '/api/photos') {
      const photos = [...states.entries()]
        .filter(([, status]) => status !== 'pending')
        .map(([id, status]) => ({
          id,
          filename: id === 'photo-ready' ? 'garden.jpg' : 'bridge.jpg',
          status,
          thumbnailUrl:
            status === 'ready' ? `/api/photos/${id}/thumbnail` : undefined,
          originalUrl:
            status === 'ready' ? `/api/photos/${id}/original` : undefined,
        }));
      return jsonResponse({ photos, total: photos.length });
    }

    if (path === '/api/uploads' && init?.method === 'POST') {
      const { filename } = JSON.parse(String(init.body));
      const id = filename === 'garden.jpg' ? 'photo-ready' : 'photo-bridge';
      states.set(id, 'pending');
      return jsonResponse(
        {
          id,
          photoId: id,
          status: 'pending',
          uploadUrl: `https://uploads.example.test/${id}`,
        },
        201,
      );
    }

    if (
      path.startsWith('/api/uploads/') &&
      (!init?.method || init.method === 'GET')
    ) {
      const id = path.slice('/api/uploads/'.length);
      return jsonResponse({ id, photoId: id, status: states.get(id) });
    }

    throw new Error(`Unexpected request: ${init?.method ?? 'GET'} ${path}`);
  });

  vi.stubGlobal('fetch', fetch);
  return { ...transport, states, fetch };
}

function jpegFile(filename = 'garden.jpg') {
  return new File(['fictional image bytes'], filename, { type: 'image/jpeg' });
}

async function startBatch(files: File[]) {
  vi.useFakeTimers();
  const batch = mockBatch();
  let view!: ReturnType<typeof render>;

  await act(async () => {
    view = showGallery();
  });
  await act(async () => {
    fireEvent.change(screen.getByLabelText(/upload photos/i), {
      target: { files },
    });
  });

  return { ...batch, view };
}

async function completeTransfer(request: XMLHttpRequest, status: number) {
  await act(async () => {
    finishUpload(request, status);
  });
}

async function advancePolling() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(5_000);
  });
}

it('uploads the selected original directly to authorized storage', async () => {
  const file = jpegFile();
  const batch = await startBatch([file]);

  expect(batch.urls).toEqual(['https://uploads.example.test/photo-ready']);
  expect(batch.send).toHaveBeenCalledWith(file);
});

it('shows upload progress for the selected file', async () => {
  const batch = await startBatch([jpegFile()]);

  act(() =>
    batch.requests[0].upload.dispatchEvent(
      new ProgressEvent('progress', {
        lengthComputable: true,
        loaded: 5,
        total: 10,
      }),
    ),
  );

  const progress = screen.getByRole('progressbar', { name: 'garden.jpg' });
  expect(
    Number(
      progress.getAttribute('aria-valuenow') ?? progress.getAttribute('value'),
    ),
  ).toBe(50);
});

it('shows processing after the original finishes uploading', async () => {
  const batch = await startBatch([jpegFile()]);
  batch.states.set('photo-ready', 'processing');

  await completeTransfer(batch.requests[0], 200);
  await advancePolling();

  const photo = screen.getByRole('group', { name: 'garden.jpg' });
  expect(within(photo).getByText(/processing/i)).toBeInTheDocument();
  expect(within(photo).queryByRole('img')).not.toBeInTheDocument();
});

it('shows the thumbnail when background processing finishes', async () => {
  const batch = await startBatch([jpegFile()]);
  batch.states.set('photo-ready', 'processing');
  await completeTransfer(batch.requests[0], 200);
  await advancePolling();

  batch.states.set('photo-ready', 'ready');
  await advancePolling();

  expect(screen.getByRole('img', { name: 'garden.jpg' })).toHaveAttribute(
    'src',
    readyPhoto.thumbnailUrl,
  );
});

it('preserves a successful upload when another file fails', async () => {
  const batch = await startBatch([
    jpegFile('garden.jpg'),
    jpegFile('bridge.jpg'),
  ]);
  batch.states.set('photo-ready', 'ready');
  await completeTransfer(batch.requests[0], 200);

  const failedIndex = batch.urls.findIndex((url) =>
    url.endsWith('/photo-bridge'),
  );
  expect(failedIndex).toBeGreaterThanOrEqual(0);
  await completeTransfer(batch.requests[failedIndex], 503);
  await advancePolling();

  expect(screen.getByRole('img', { name: 'garden.jpg' })).toBeInTheDocument();
  expect(
    within(screen.getByRole('group', { name: 'bridge.jpg' })).getByText(
      /failed/i,
    ),
  ).toBeInTheDocument();
});

it('reports a transfer connection failure', async () => {
  const batch = await startBatch([jpegFile()]);

  await act(async () => {
    batch.requests[0].dispatchEvent(new Event('error'));
  });

  expect(
    within(screen.getByRole('group', { name: 'garden.jpg' })).getByText(
      /failed/i,
    ),
  ).toBeInTheDocument();
});

it('does not transfer a file when upload authorization fails', async () => {
  const transport = mockUploads();
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) =>
      String(input) === '/api/photos'
        ? jsonResponse({ photos: [], total: 0 })
        : jsonResponse({ error: 'Upload authorization unavailable' }, 503),
    ),
  );
  showGallery();

  fireEvent.change(await screen.findByLabelText(/upload photos/i), {
    target: { files: [jpegFile()] },
  });

  expect(await screen.findByRole('alert')).toHaveTextContent(/upload/i);
  expect(transport.send).not.toHaveBeenCalled();
});

it('stops polling after leaving the gallery', async () => {
  const batch = await startBatch([jpegFile()]);
  batch.states.set('photo-ready', 'processing');
  await completeTransfer(batch.requests[0], 200);
  await advancePolling();

  batch.view.unmount();
  const requestsAtUnmount = batch.fetch.mock.calls.length;
  await advancePolling();

  expect(batch.fetch).toHaveBeenCalledTimes(requestsAtUnmount);
});
