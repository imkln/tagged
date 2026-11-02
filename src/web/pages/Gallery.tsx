import Brand from '../components/Brand';
import './Gallery.css';

function PhotoIcon() {
  return (
    <svg viewBox="0 0 48 48" fill="none" aria-hidden="true">
      <rect x="8" y="9" width="32" height="30" rx="5" />
      <circle cx="30" cy="19" r="3" />
      <path d="m9 32 10-10 10 11 5-5 6 7" />
    </svg>
  );
}

export default function Gallery({ email }: { email: string }) {
  return (
    <div className="app-layout">
      <aside className="sidebar">
        <Brand />
        <nav aria-label="Library">
          <a href="/gallery" className="nav-item active" aria-current="page">
            <span aria-hidden="true">▧</span> Gallery{' '}
            <span className="nav-count">0</span>
          </a>
        </nav>
        <div className="sidebar-note">
          <span className="privacy-dot" /> Private library
          <p>Only you have access to your photos.</p>
        </div>
        <div className="account">
          <span className="avatar" aria-hidden="true">
            {email.charAt(0).toUpperCase()}
          </span>
          <span className="account-email">{email}</span>
          <form method="post" action="/auth/logout">
            <button className="sign-out" type="submit">
              Sign out
            </button>
          </form>
        </div>
      </aside>
      <main className="gallery">
        <header className="gallery-header">
          <div>
            <p className="eyebrow">Your collection</p>
            <h1>Gallery</h1>
          </div>
          <span className="private-badge">
            <span className="privacy-dot" /> Private
          </span>
        </header>
        <div className="gallery-toolbar">
          <span>All photos</span>
          <span className="photo-count">0 photos</span>
        </div>
        <section className="empty-library" aria-labelledby="empty-title">
          <div className="empty-stack" aria-hidden="true">
            <div />
            <div />
            <div>
              <PhotoIcon />
            </div>
          </div>
          <p className="eyebrow">A fresh start</p>
          <h2 id="empty-title">Your library starts here</h2>
          <p>No photos yet. This is where your collection will live.</p>
        </section>
        <footer className="gallery-footer">
          <span className="privacy-dot" /> A little space for your world.
        </footer>
      </main>
    </div>
  );
}
