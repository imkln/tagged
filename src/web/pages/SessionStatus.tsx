import Brand from '../components/Brand';
import './SessionStatus.css';

type Props = {
  failed: boolean;
  onRetry: () => void;
};

export default function SessionStatus({ failed, onRetry }: Props) {
  return (
    <main className="status-page">
      <Brand />
      {failed ? (
        <section role="alert">
          <h1>Unable to reach Tagged</h1>
          <p>Check your connection and try again.</p>
          <button className="button" onClick={onRetry}>
            Try again
          </button>
        </section>
      ) : (
        <p role="status">Opening your library…</p>
      )}
    </main>
  );
}
