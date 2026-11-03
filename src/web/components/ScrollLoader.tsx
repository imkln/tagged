import { useEffect, useRef } from 'react';
import './ScrollLoader.css';

export default function ScrollLoader({
  hasMore,
  loading,
  error,
  onLoadMore,
}: {
  hasMore: boolean;
  loading: boolean;
  error: string;
  onLoadMore: (retry?: boolean) => Promise<void>;
}) {
  const sentinel = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (
      !hasMore ||
      loading ||
      error ||
      !sentinel.current ||
      typeof IntersectionObserver === 'undefined'
    )
      return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) void onLoadMore();
      },
      { rootMargin: '400px' },
    );
    observer.observe(sentinel.current);

    return () => observer.disconnect();
  }, [hasMore, loading, error, onLoadMore]);

  if (!hasMore) return null;

  return (
    <div className="scroll-loader" ref={sentinel}>
      {error ? (
        <div role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => void onLoadMore(true)}>
            Retry loading more
          </button>
        </div>
      ) : loading ? (
        <p role="status">Loading more photos…</p>
      ) : (
        <button type="button" onClick={() => void onLoadMore()}>
          Load more
        </button>
      )}
    </div>
  );
}
