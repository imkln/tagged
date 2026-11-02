import Brand from '../components/Brand';
import './SignIn.css';

export default function SignIn({ denied }: { denied: boolean }) {
  return (
    <main className="welcome">
      <header className="welcome-header">
        <Brand />
        <span className="privacy-label">Your private photo library</span>
      </header>
      <section className="welcome-content">
        <div className="welcome-copy">
          <p className="eyebrow">A place for your perspective</p>
          <h1>
            {denied ? (
              'This library is private'
            ) : (
              <>
                Every photo.
                <br />A little easier to find.
              </>
            )}
          </h1>
          <p className="intro">
            {denied
              ? 'This Google account does not have access. Sign in with the account authorized for this library.'
              : 'Your travels, everyday discoveries, and the moments worth keeping. Together in one personal library.'}
          </p>
          <a className="button google-button" href="/auth/login">
            <span className="google-mark" aria-hidden="true">
              G
            </span>
            {denied ? 'Try another Google account' : 'Continue with Google'}
          </a>
          <p className="access-note">
            Access is restricted to the library owner.
          </p>
        </div>
        <div className="photo-art" aria-hidden="true">
          <div className="art-photo art-photo-back" />
          <div className="art-photo art-photo-middle" />
          <div className="art-photo art-photo-front">
            <div className="art-sun" />
            <div className="art-hill far" />
            <div className="art-hill near" />
          </div>
          <span className="art-caption">Room for the moments that matter.</span>
        </div>
      </section>
      <footer className="welcome-footer">
        Your photos. Your library. Your point of view.
      </footer>
    </main>
  );
}
