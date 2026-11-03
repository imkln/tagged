import Brand from './Brand';
import './AppHeader.css';

export default function AppHeader({ email }: { email?: string }) {
  return (
    <header className="app-header">
      <Brand />
      {email && (
        <form method="post" action="/auth/logout">
          <button
            className="sign-out"
            type="submit"
            title={`Sign out of ${email}`}
          >
            Sign out
          </button>
        </form>
      )}
    </header>
  );
}
