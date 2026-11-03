import { useEffect, useId, useState } from 'react';
import type { Tag } from '../hooks/useLibrary';
import './TagSearch.css';

export default function TagSearch({
  selected,
  onChange,
}: {
  selected: Tag[];
  onChange: (tags: Tag[]) => void;
}) {
  const [query, setQuery] = useState('');
  const [options, setOptions] = useState<Tag[]>([]);
  const [active, setActive] = useState(-1);
  const [error, setError] = useState('');
  const listId = useId();

  useEffect(() => {
    setOptions([]);
    setActive(-1);
    setError('');
    if (!query.trim()) return;

    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch(
          `/api/tags?q=${encodeURIComponent(query)}`,
          {
            credentials: 'same-origin',
            signal: controller.signal,
          },
        );
        if (!response.ok) throw new Error('Unable to load tags.');

        const body = await response.json();
        if (!Array.isArray(body.tags)) throw new Error('Unable to load tags.');

        if (!controller.signal.aborted) {
          setOptions(
            body.tags.filter(
              (tag: Tag) => !selected.some((item) => item.id === tag.id),
            ),
          );
        }
      } catch {
        if (!controller.signal.aborted) setError('Unable to load tags.');
      }
    })();

    return () => controller.abort();
  }, [query, selected]);

  function choose(tag: Tag) {
    onChange([...selected, tag]);
    setQuery('');
    setOptions([]);
    setActive(-1);
  }

  return (
    <div className="tag-search">
      <div className="selected-tags">
        {selected.map((tag) => (
          <button
            key={tag.id}
            type="button"
            aria-label={`Remove ${tag.name} filter`}
            onClick={() =>
              onChange(selected.filter((item) => item.id !== tag.id))
            }
          >
            {tag.name} ×
          </button>
        ))}
        {selected.length > 0 && (
          <button type="button" onClick={() => onChange([])}>
            Reset filters
          </button>
        )}
      </div>
      <div className="tag-input">
        <input
          role="combobox"
          aria-label="Search tags"
          placeholder="Search tags"
          aria-autocomplete="list"
          aria-expanded={options.length > 0}
          aria-controls={listId}
          aria-activedescendant={
            active >= 0 ? `${listId}-${active}` : undefined
          }
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown' && options.length) {
              event.preventDefault();
              setActive((index) => (index + 1) % options.length);
            }

            if (event.key === 'ArrowUp' && options.length) {
              event.preventDefault();
              setActive(
                (index) => (index - 1 + options.length) % options.length,
              );
            }

            if (event.key === 'Enter' && options[active]) {
              event.preventDefault();
              choose(options[active]);
            }

            if (event.key === 'Escape') {
              setOptions([]);
              setActive(-1);
            }
          }}
        />
        {options.length > 0 && (
          <ul id={listId} role="listbox" aria-label="Matching tags">
            {options.map((tag, index) => (
              <li
                key={tag.id}
                id={`${listId}-${index}`}
                role="option"
                aria-selected={index === active}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => choose(tag)}
              >
                {tag.name}
              </li>
            ))}
          </ul>
        )}
      </div>
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
