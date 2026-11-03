import { useEffect, useId, useRef, useState } from 'react';
import type { Tag } from '../hooks/useLibrary';
import './TagSearch.css';

export default function TagSearch({
  selected,
  onChange,
}: {
  selected: Tag[];
  onChange: (tags: Tag[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [options, setOptions] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const anchor = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const [active, setActive] = useState(-1);
  const pickerId = useId();

  useEffect(() => {
    if (!open) return;

    const controller = new AbortController();
    setLoading(true);
    setError('');
    setOptions([]);
    setActive(-1);

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

        if (!controller.signal.aborted) setOptions(body.tags);
      } catch {
        if (!controller.signal.aborted) setError('Unable to load tags.');
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    })();

    return () => controller.abort();
  }, [open, query]);

  useEffect(() => {
    if (!open) return;

    function dismiss(event: PointerEvent) {
      if (
        event.target instanceof Node &&
        !anchor.current?.contains(event.target)
      )
        setOpen(false);
    }

    document.addEventListener('pointerdown', dismiss);
    return () => document.removeEventListener('pointerdown', dismiss);
  }, [open]);

  const available = options.filter(
    (tag) => !selected.some((item) => item.id === tag.id),
  );

  function choose(tag: Tag) {
    onChange([...selected, tag]);
    setQuery('');
    setOpen(false);
    setActive(-1);
    input.current?.focus();
  }

  function remove(id: string) {
    onChange(selected.filter((tag) => tag.id !== id));
    setActive(-1);
    input.current?.focus();
  }

  return (
    <div
      className="tag-search"
      ref={anchor}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null))
          setOpen(false);
      }}
    >
      <div className="tag-search-field" onClick={() => input.current?.focus()}>
        <svg
          className="search-symbol"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.6"
          aria-hidden="true"
        >
          <circle cx="10.5" cy="10.5" r="6.5" />
          <path d="m15.5 15.5 5 5" />
        </svg>
        {selected.map((tag) => (
          <span className="search-tag" key={tag.id}>
            <TagPath tag={tag} />
            <button
              type="button"
              aria-label={`Remove ${tag.name} filter`}
              onClick={(event) => {
                event.stopPropagation();
                remove(tag.id);
              }}
            >
              ×
            </button>
          </span>
        ))}
        <input
          ref={input}
          role="combobox"
          aria-label="Search tags"
          placeholder={selected.length ? 'Add a tag…' : 'Search tags…'}
          value={query}
          aria-expanded={open}
          aria-controls={open ? pickerId : undefined}
          aria-autocomplete="list"
          aria-activedescendant={
            open && available[active] ? `${pickerId}-${active}` : undefined
          }
          onClick={() => setOpen(true)}
          onChange={(event) => {
            setQuery(event.target.value);
            setOpen(true);
            setActive(-1);
          }}
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              event.stopPropagation();
              setOpen(false);
              setActive(-1);
            }

            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
              event.preventDefault();
              setOpen(true);
              if (available.length) {
                const direction = event.key === 'ArrowDown' ? 1 : -1;
                setActive((index) =>
                  index < 0
                    ? direction === 1
                      ? 0
                      : available.length - 1
                    : (index + direction + available.length) % available.length,
                );
              }
            }

            if (
              event.key === 'Enter' &&
              open &&
              available[active] &&
              !loading &&
              !error
            ) {
              event.preventDefault();
              choose(available[active]);
            }

            if (event.key === 'Backspace' && !query && selected.length) {
              remove(selected[selected.length - 1].id);
            }
          }}
        />
        {selected.length > 0 && (
          <button
            className="clear-tag-search"
            type="button"
            aria-label="Clear filters"
            onClick={(event) => {
              event.stopPropagation();
              onChange([]);
              setQuery('');
              setOpen(false);
              setActive(-1);
            }}
          >
            ×
          </button>
        )}
      </div>
      {open && (
        <div className="tag-suggestions">
          {loading ? (
            <p role="status">Loading tags…</p>
          ) : error ? (
            <p role="alert">{error}</p>
          ) : null}
          <ul
            role="listbox"
            id={pickerId}
            aria-label="Tag suggestions"
            aria-busy={loading}
          >
            {!loading &&
              !error &&
              available.map((tag, index) => (
                <li
                  key={tag.id}
                  role="option"
                  id={`${pickerId}-${index}`}
                  aria-selected={index === active}
                  aria-label={(tag.path?.length ? tag.path : [tag.name]).join(
                    ' › ',
                  )}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => choose(tag)}
                >
                  <span className="suggestion-path">
                    <TagPath tag={tag} />
                  </span>
                  {tag.count !== undefined && (
                    <small aria-hidden="true">{tag.count}</small>
                  )}
                </li>
              ))}
          </ul>
          {!loading && !error && !available.length && <p>No matching tags</p>}
        </div>
      )}
    </div>
  );
}

function TagPath({ tag }: { tag: Tag }) {
  return (tag.path?.length ? tag.path : [tag.name]).map((part, index) => (
    <span key={index}>
      {index > 0 && (
        <span className="path-separator" aria-hidden="true">
          ›
        </span>
      )}
      {part}
    </span>
  ));
}
