import AppHeader from '../components/AppHeader';
import './SignIn.css';

export default function SignIn({ denied }: { denied: boolean }) {
  return (
    <div className="sign-in-page">
      <AppHeader />
      <main className="sign-in-content">
        <h1>{denied ? 'This library is private' : 'Sign in'}</h1>
        <p>
          {denied
            ? 'This Google account does not have access. Try the account authorized for this library.'
            : 'Use your authorized Google account to open the library.'}
        </p>
        <a className="button" href="/auth/login">
          {denied ? 'Try another Google account' : 'Continue with Google'}
        </a>
      </main>
    </div>
  );
}
