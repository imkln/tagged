import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import App from './App';

afterEach(() => vi.unstubAllGlobals());

function mockSession(status: number, body: unknown) {
  const fetchSession = vi.fn().mockResolvedValue({
    status,
    ok: status >= 200 && status < 300,
    json: async () => body,
  });

  vi.stubGlobal('fetch', fetchSession);
}

it('offers Google sign-in without exposing the library', async () => {
  mockSession(401, { error: 'unauthorized' });

  render(<App />);

  const signIn = await screen.findByRole('link', {
    name: /continue with google/i,
  });

  expect(signIn).toHaveAttribute('href', '/auth/login');
  expect(
    screen.queryByRole('heading', { name: 'Gallery' }),
  ).not.toBeInTheDocument();
});

it('shows the owner an empty gallery and a sign-out form', async () => {
  mockSession(200, { email: 'owner@example.test' });

  render(<App />);

  expect(
    await screen.findByRole('heading', { name: 'Gallery' }),
  ).toBeInTheDocument();
  expect(screen.getByText('Your library starts here')).toBeInTheDocument();
  expect(screen.getByText('0 photos')).toBeInTheDocument();

  const signOut = screen
    .getByRole('button', { name: 'Sign out' })
    .closest('form');

  expect(signOut).toHaveAttribute('method', 'post');
  expect(signOut).toHaveAttribute('action', '/auth/logout');
});

it('denies other accounts without displaying the gallery', async () => {
  mockSession(403, { error: 'forbidden' });

  render(<App />);

  expect(
    await screen.findByRole('heading', { name: 'This library is private' }),
  ).toBeInTheDocument();

  const signIn = screen.getByRole('link', {
    name: /try another google account/i,
  });

  expect(signIn).toHaveAttribute('href', '/auth/login');
  expect(
    screen.queryByRole('heading', { name: 'Gallery' }),
  ).not.toBeInTheDocument();
});

it('lets the user retry after a connection failure', async () => {
  const fetchSession = vi
    .fn()
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValue({
      status: 200,
      ok: true,
      json: async () => ({ email: 'owner@example.test' }),
    });

  vi.stubGlobal('fetch', fetchSession);

  render(<App />);

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Unable to reach Tagged',
  );

  fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

  expect(
    await screen.findByRole('heading', { name: 'Gallery' }),
  ).toBeInTheDocument();
});
