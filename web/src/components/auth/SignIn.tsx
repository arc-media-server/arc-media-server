import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { AuthScreen, Card, fieldClass, Label } from './Screen';

/** Signing in with the household password (away from home, or when that's always required), or
 * as a friend with their own name and password. */
export function SignIn({ asFriend = false }: { asFriend?: boolean }) {
  const qc = useQueryClient();
  const [friend, setFriend] = useState(asFriend);
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const login = useMutation({
    mutationFn: () => (friend ? api.friendLogin(name, password) : api.login(password)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['auth'] }),
  });
  const switchTo = (f: boolean) => {
    setFriend(f);
    setPassword('');
    login.reset();
  };
  return (
    <AuthScreen>
      <Card>
        <h1 className="text-2xl font-bold">Sign in</h1>
        <p className="mt-1.5 text-sm text-muted">{friend ? 'Enter the name and password you chose with your invite.' : 'Enter the password for this Arc server.'}</p>
        <form
          className="mt-6 space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            login.mutate();
          }}
        >
          {friend && (
            <label className="block">
              <Label>Name</Label>
              <input autoFocus name="name" value={name} onChange={(e) => setName(e.target.value)} className={fieldClass} autoComplete="username" />
            </label>
          )}
          <label className="block">
            <Label>Password</Label>
            <input
              autoFocus={!friend}
              type="password"
              name="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={fieldClass}
              autoComplete="current-password"
            />
          </label>
          {login.error && <p className="text-sm text-warning">{login.error.message}</p>}
          <Button type="submit" variant="primary" size="lg" className="w-full" disabled={!password || (friend && !name.trim()) || login.isPending}>
            {login.isPending && <Loader2 className="animate-spin" />} Sign In
          </Button>
        </form>
        <button type="button" onClick={() => switchTo(!friend)} className="mt-4 w-full text-center text-sm text-muted transition-colors hover:text-foreground">
          {friend ? 'Sign in with the household password' : 'Invited by a friend? Sign in with your name'}
        </button>
      </Card>
    </AuthScreen>
  );
}
