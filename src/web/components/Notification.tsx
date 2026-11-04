import { useEffect, useState } from 'react';
import './Notification.css';

export default function Notification({
  message,
  role = 'alert',
}: {
  message: string;
  role?: 'alert' | 'status';
}) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    setVisible(Boolean(message));
    if (!message || role === 'status') return;

    const timeout = window.setTimeout(() => setVisible(false), 4000);
    return () => window.clearTimeout(timeout);
  }, [message, role]);

  return visible && message ? (
    <div className="notification" role={role}>
      {message}
    </div>
  ) : null;
}
