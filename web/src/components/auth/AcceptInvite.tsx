import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { AuthScreen, Card, fieldClass, Label } from './Screen';

/** A friend's invite link: they choose a password, then they're in. */
export function AcceptInvite({ token }: { token: string }) {
  const invite = useQuery({ queryKey: ['invite', token], queryFn: () => api.invite(token), retry: false });
  const [password, setPassword] = useState('');
  const accept = useMutation({
    mutationFn: () => api.acceptInvite(token, password),
    // A full load, so the app starts afresh at Home as them.
    onSuccess: () => window.location.replace('/'),
  });
  return (
    <AuthScreen>
      <Card>
        {invite.error ? (
          <>
            <h1 className="text-2xl font-bold">Invite link</h1>
            <p className="mt-1.5 text-sm text-muted">{invite.error.message}</p>
            <Button variant="secondary" size="lg" className="mt-6 w-full" onClick={() => (window.location.href = '/')}>
              Sign In
            </Button>
          </>
        ) : (
          <>
            <h1 className="text-2xl font-bold">{invite.data ? `Welcome, ${invite.data.name}` : 'Welcome'}</h1>
            <p className="mt-1.5 text-sm text-muted">
              You&apos;ve been invited to watch on this Arc server. Choose a password; you&apos;ll sign in with your name and it from now on.
            </p>
            <form
              className="mt-6 space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                accept.mutate();
              }}
            >
              <label className="block">
                <Label>Password</Label>
                <input
                  autoFocus
                  type="password"
                  name="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className={fieldClass}
                  autoComplete="new-password"
                  placeholder="At least 8 characters"
                />
              </label>
              {accept.error && <p className="text-sm text-warning">{accept.error.message}</p>}
              <Button type="submit" variant="primary" size="lg" className="w-full" disabled={password.length < 8 || !invite.data || accept.isPending}>
                {accept.isPending && <Loader2 className="animate-spin" />} Start Watching
              </Button>
            </form>
          </>
        )}
      </Card>
    </AuthScreen>
  );
}
