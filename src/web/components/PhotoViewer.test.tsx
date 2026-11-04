import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import PhotoViewer from './PhotoViewer';

it('a loading failure should show a temporary message with no actions', async () => {
  vi.useFakeTimers();
  const photo = {
    id: 'sample',
    filename: 'sample.jpg',
    status: 'ready',
    originalUrl: '/sample.jpg',
  };

  try {
    render(
      <PhotoViewer
        photo={photo}
        photos={[photo]}
        total={15}
        hasMore
        loadingMore={false}
        pageError="Unable to load more photos."
        onLoadMore={vi.fn(async () => {})}
        onClose={() => {}}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Next photo' }));

    const message = within(screen.getByRole('dialog')).getByRole('alert');
    expect(message).toHaveTextContent('Unable to load more photos.');
    expect(within(message).queryByRole('button')).not.toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  } finally {
    vi.useRealTimers();
  }
});

it('the viewer should show the library total rather than the number of loaded photos', () => {
  const photo = {
    id: 'sample',
    filename: 'sample.jpg',
    status: 'ready',
    originalUrl: '/sample.jpg',
  };

  render(
    <PhotoViewer
      photo={photo}
      photos={[photo]}
      total={15}
      hasMore
      loadingMore={false}
      pageError=""
      onLoadMore={vi.fn(async () => {})}
      onClose={() => {}}
    />,
  );

  expect(
    within(screen.getByRole('dialog')).getByText('1 / 15'),
  ).toBeInTheDocument();
});
