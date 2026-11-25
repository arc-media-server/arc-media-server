import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { AuthScreen, Card, fieldClass, Label } from './Screen';

/** Signing in with the household password (away from home, or when that's always required), or
 * as a friend with their own name and password. */
export function SignIn({ asFriend = false }: { asFriend?: boolean }) {
  const { t } = useTranslation(['auth', 'common']);
  const qc = useQueryClient();
  const [friend, setFriend] = useState(asFriend);
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const login = useMutation({
    mutationFn: () => (friend ? api.friendLogin(name, password) : api.login(password)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['auth'] }),
  });
  const switchTo = (f: boolean) => {
    if (f === friend) return;
    setFriend(f);
    setPassword('');
    login.reset();
  };
  return (
    <AuthScreen>
      <Card>
        <h1 className="text-2xl font-bold">{t('auth:signIn.heading')}</h1>
        <div role="group" aria-label={t('auth:signIn.method')} className="mt-5 grid grid-cols-2 gap-2">
          {[false, true].map((mode) => (
            <Button
              key={String(mode)}
              type="button"
              variant={friend === mode ? 'primary' : 'secondary'}
              aria-pressed={friend === mode}
              disabled={login.isPending}
              className={cn('h-auto min-h-10 w-full whitespace-normal', friend !== mode && 'ring-1 ring-border')}
              onClick={() => switchTo(mode)}
            >
              {mode ? t('auth:signIn.friendOption') : t('auth:signIn.serverPasswordOption')}
            </Button>
          ))}
        </div>
        <p className="mt-4 text-sm leading-relaxed text-muted">
          {friend ? t('auth:signIn.friendOptionDescription') : t('auth:signIn.serverPasswordDescription')}
        </p>
        <form
          className="mt-6 space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            login.mutate();
          }}
        >
          {friend && (
            <label className="block">
              <Label>{t('auth:name')}</Label>
              <input autoFocus name="name" value={name} onChange={(e) => setName(e.target.value)} className={fieldClass} autoComplete="username" />
            </label>
          )}
          <label className="block">
            <Label>{t('auth:password')}</Label>
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
            {login.isPending && <Loader2 className="animate-spin" />}{' '}{t('common:actions.signIn')}</Button>
        </form>
        {friend && <p className="mt-4 text-sm leading-relaxed text-muted">{t('auth:signIn.inviteHelp')}</p>}
      </Card>
    </AuthScreen>
  );
}
