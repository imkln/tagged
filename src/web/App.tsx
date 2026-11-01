import { useSession } from './hooks/useSession';
import Gallery from './pages/Gallery';
import SignIn from './pages/SignIn';
import SessionStatus from './pages/SessionStatus';

export default function App() {
  const { session, retry } = useSession();

  if (session.status === 'signed-in') {
    return <Gallery email={session.email} />;
  }

  if (session.status === 'signed-out' || session.status === 'denied') {
    return <SignIn denied={session.status === 'denied'} />;
  }

  return <SessionStatus failed={session.status === 'error'} onRetry={retry} />;
}
