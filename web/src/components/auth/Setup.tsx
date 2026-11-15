import { useTranslation } from 'react-i18next';
import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ExternalLink, Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { AuthScreen, Card, fieldClass, Label } from './Screen';

/** First run: create the admin profile, choose sign-in at home, and connect TMDB/TheTVDB. */
export function Setup() {
  const { t } = useTranslation(['auth', 'common']);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [step, setStep] = useState<'account' | 'metadata'>('account');
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [bypass, setBypass] = useState(true);
  const [tmdb, setTmdb] = useState('');
  const [tvdb, setTvdb] = useState('');

  const accountError =
    password && password.length < 8 ? t('auth:passwordHint') : confirm && confirm !== password ? t('auth:setup.passwordMismatch') : '';
  const setup = useMutation({
    mutationFn: () => api.setup({ name, password, local_bypass: bypass, tmdb_api_key: tmdb, tvdb_api_key: tvdb, region: browserRegion() }),
    onSuccess: async () => {
      // Straight on to adding the first library.
      navigate('/libraries?add=1', { replace: true });
      await qc.invalidateQueries();
    },
  });

  const next = (e: FormEvent) => {
    e.preventDefault();
    if (name.trim() && password.length >= 8 && password === confirm) setStep('metadata');
  };

  return (
    <AuthScreen>
      {step === 'account' ? (
        <Card>
          <h1 className="text-2xl font-bold">{t('auth:setup.heading')}</h1>
          <p className="mt-1.5 text-sm text-muted">{t('auth:setup.description')}</p>
          <form onSubmit={next} className="mt-6 space-y-4">
            <label className="block">
              <Label>{t('auth:setup.yourName')}</Label>
              <input autoFocus value={name} onChange={(e) => setName(e.target.value)} className={fieldClass} autoComplete="name" />
            </label>
            <label className="block">
              <Label>{t('auth:password')}</Label>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className={fieldClass} autoComplete="new-password" aria-describedby={accountError ? 'setup-account-error' : undefined} aria-invalid={!!accountError && password.length < 8} />
            </label>
            <label className="block">
              <Label>{t('auth:setup.confirmPassword')}</Label>
              <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} className={fieldClass} autoComplete="new-password" aria-describedby={accountError ? 'setup-account-error' : undefined} aria-invalid={!!confirm && confirm !== password} />
            </label>
            {accountError && <p id="setup-account-error" role="alert" className="text-sm text-warning">{accountError}</p>}
            <div className="flex items-start gap-3 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{t('auth:setup.bypass')}</p>
                <p className="mt-0.5 text-xs leading-relaxed text-muted">{t('auth:setup.bypassDescription')}</p>
              </div>
              <Switch checked={bypass} onCheckedChange={setBypass} aria-label={t('auth:setup.bypass')} />
            </div>
            <Button type="submit" variant="primary" size="lg" className="w-full" disabled={!name.trim() || password.length < 8 || password !== confirm}>{t('common:actions.continue')}</Button>
          </form>
        </Card>
      ) : (
        <Card>
          <h1 className="text-2xl font-bold">{t('auth:setup.providersHeading')}</h1>
          <p className="mt-1.5 text-sm text-muted">{t('auth:setup.providersDescription')}</p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setup.mutate();
            }}
            className="mt-6 space-y-4"
          >
            <label className="block">
              <Label>{t('auth:setup.tmdbKey')}</Label>
              <input value={tmdb} onChange={(e) => setTmdb(e.target.value)} className={fieldClass} placeholder={t('auth:setup.keyPlaceholder')} spellCheck={false} />
              <KeyLink href="https://www.themoviedb.org/settings/api">{t('auth:setup.getTmdbKey')}</KeyLink>
            </label>
            <label className="block">
              <Label>{t('auth:setup.tvdbKey')}</Label>
              <input value={tvdb} onChange={(e) => setTvdb(e.target.value)} className={fieldClass} placeholder={t('auth:setup.keyPlaceholder')} spellCheck={false} />
              <KeyLink href="https://thetvdb.com/api-information">{t('auth:setup.getTvdbKey')}</KeyLink>
            </label>
            {!tmdb && !tvdb && <p className="text-xs leading-relaxed text-muted">{t('auth:setup.withoutKeys')}</p>}
            {setup.error && <p role="alert" className="text-sm text-warning">{setup.error.message}</p>}
            <div className="flex gap-3 pt-1">
              <Button type="button" variant="ghost" size="lg" onClick={() => setStep('account')} disabled={setup.isPending}>{t('common:actions.back')}</Button>
              <Button type="submit" variant="primary" size="lg" className="flex-1" disabled={setup.isPending}>
                {setup.isPending && <Loader2 className="animate-spin" />}
                {tmdb || tvdb ? t('auth:setup.finish') : t('auth:setup.skip')}
              </Button>
            </div>
          </form>
        </Card>
      )}
    </AuthScreen>
  );
}

/** The country from the browser's language setting (en-GB → GB), for age ratings. Changeable in Settings. */
function browserRegion(): string | undefined {
  try {
    return new Intl.Locale(navigator.language).maximize().region;
  } catch {
    return undefined;
  }
}

function KeyLink({ href, children }: { href: string; children: string }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="mt-1.5 inline-flex items-center gap-1 text-xs text-accent hover:underline">
      {children} <ExternalLink className="size-3" />
    </a>
  );
}
