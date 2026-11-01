import './Brand.css';

export default function Brand() {
  return (
    <a className="brand" href="/" aria-label="Tagged home">
      <span className="brand-mark" aria-hidden="true">
        <span />
        <span />
      </span>
      Tagged
    </a>
  );
}
