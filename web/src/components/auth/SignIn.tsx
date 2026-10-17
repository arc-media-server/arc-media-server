import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { AuthScreen, Card, fieldClass, Label } from './Screen';

/** Signing in with the household password (away from home, or when that's always required). */
export function SignIn() {
  const qc = useQueryClient();
  const [password, setPassword] = useState('');
  const login = useMutation({
    mutationFn: () => api.login(password),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['auth'] }),
  });
  return (
    <AuthScreen>
      <Card>
        <h1 className="text-2xl font-bold">Sign in</h1>
        <p className="mt-1.5 text-sm text-muted">Enter the password for this Arc server.</p>
        <form
          className="mt-6 space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            login.mutate();
          }}
        >
          <label className="block">
            <Label>Password</Label>
            <input autoFocus type="password" value={password} onChange={(e) => setPassword(e.target.value)} className={fieldClass} autoComplete="current-password" />
          </label>
          {login.error && <p className="text-sm text-warning">{login.error.message}</p>}
          <Button type="submit" variant="primary" size="lg" className="w-full" disabled={!password || login.isPending}>
            {login.isPending && <Loader2 className="animate-spin" />} Sign In
          </Button>
        </form>
      </Card>
    </AuthScreen>
  );
}
