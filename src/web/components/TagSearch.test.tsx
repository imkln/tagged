import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { useState } from 'react';
import { afterEach, expect, it, vi } from 'vitest';
import type { Tag } from '../hooks/useLibrary';
import TagSearch from './TagSearch';

const tags = [
  { id: 'italy', name: 'Italy' },
  { id: 'rome', name: 'Rome', path: ['Italy', 'Rome'] },
];

function showSearch() {
  function Search() {
    const [selected, setSelected] = useState<Tag[]>([]);

    return <TagSearch selected={selected} onChange={setSelected} />;
  }

  return render(<Search />);
}

function respond(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

it('supports keyboard tag selection and dismissal', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation(async () => respond({ tags })),
  );
  showSearch();
  const input = screen.getByRole('combobox', { name: 'Search tags' });

  fireEvent.click(input);
  await screen.findByRole('option', { name: 'Italy' });
  fireEvent.keyDown(input, { key: 'ArrowDown' });
  fireEvent.keyDown(input, { key: 'ArrowDown' });
  fireEvent.keyDown(input, { key: 'ArrowUp' });
  fireEvent.keyDown(input, { key: 'Enter' });

  expect(
    screen.getByRole('button', { name: 'Remove Italy filter' }),
  ).toBeInTheDocument();
  expect(input).toHaveFocus();

  fireEvent.click(input);
  await screen.findByRole('option', { name: 'Italy › Rome' });
  fireEvent.keyDown(input, { key: 'Escape' });

  expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

  fireEvent.keyDown(input, { key: 'Backspace' });

  expect(
    screen.queryByRole('button', { name: 'Remove Italy filter' }),
  ).not.toBeInTheDocument();

  fireEvent.click(input);
  await screen.findByRole('option', { name: 'Italy' });
  fireEvent.pointerDown(document.body);

  expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
});

it('shows nested tag paths', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation(async () => respond({ tags })),
  );
  showSearch();

  fireEvent.click(screen.getByRole('combobox', { name: 'Search tags' }));
  fireEvent.click(await screen.findByRole('option', { name: 'Italy › Rome' }));

  const chip = screen.getByRole('button', {
    name: 'Remove Rome filter',
  }).parentElement!;
  expect(within(chip).getByText('Italy')).toBeInTheDocument();
  expect(within(chip).getByText('Rome')).toBeInTheDocument();
});

it('handles empty and failed tag searches', async () => {
  const fetch = vi.fn(async (input: RequestInfo | URL) => {
    const query = new URL(
      String(input),
      'https://example.test',
    ).searchParams.get('q');

    if (query === 'missing') return respond({ tags: [] });
    if (query === 'failed') return respond({}, 503);

    return respond({ tags });
  });
  vi.stubGlobal('fetch', fetch);
  showSearch();
  const input = screen.getByRole('combobox', { name: 'Search tags' });

  fireEvent.click(input);
  fireEvent.click(await screen.findByRole('option', { name: 'Italy' }));
  fireEvent.change(input, { target: { value: 'missing' } });

  expect(await screen.findByText('No matching tags')).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Remove Italy filter' }),
  ).toBeInTheDocument();

  fireEvent.change(input, { target: { value: 'failed' } });

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Unable to load tags.',
  );
  expect(
    screen.getByRole('button', { name: 'Remove Italy filter' }),
  ).toBeInTheDocument();

  fireEvent.change(input, { target: { value: '' } });

  await waitFor(() =>
    expect(screen.queryByRole('alert')).not.toBeInTheDocument(),
  );
  expect(
    await screen.findByRole('option', { name: 'Italy › Rome' }),
  ).toBeInTheDocument();
});
