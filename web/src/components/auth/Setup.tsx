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
    password && password.length < 8 ? 'At least 8 characters' : confirm && confirm !== password ? "The passwords don't match" : '';
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
          <h1 className="text-2xl font-bold">Welcome to Arc</h1>
          <p className="mt-1.5 text-sm text-muted">First, set up your profile. It's the admin: it manages libraries and settings.</p>
          <form onSubmit={next} className="mt-6 space-y-4">
            <label className="block">
              <Label>Your name</Label>
              <input autoFocus value={name} onChange={(e) => setName(e.target.value)} className={fieldClass} autoComplete="name" />
            </label>
            <label className="block">
              <Label>Password</Label>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className={fieldClass} autoComplete="new-password" />
            </label>
            <label className="block">
              <Label>Password again</Label>
              <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} className={fieldClass} autoComplete="new-password" />
            </label>
            {accountError && <p className="text-sm text-warning">{accountError}</p>}
            <div className="flex items-start gap-3 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Skip the password at home</p>
                <p className="mt-0.5 text-xs leading-relaxed text-muted">
                  On your home network, just pick a profile. Away from home the password is always needed, and your profile still asks
                  for it (or its PIN) so others in the house can't change settings.
                </p>
              </div>
              <Switch checked={bypass} onCheckedChange={setBypass} aria-label="Skip the password at home" />
            </div>
            <Button type="submit" variant="primary" size="lg" className="w-full" disabled={!name.trim() || password.length < 8 || password !== confirm}>
              Continue
            </Button>
          </form>
        </Card>
      ) : (
        <Card>
          <h1 className="text-2xl font-bold">Connect to TMDB and TheTVDB</h1>
          <p className="mt-1.5 text-sm text-muted">
            Arc uses them to recognise your movies and shows and fetch artwork and details. Both have free API keys; you can add them
            later in Settings.
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setup.mutate();
            }}
            className="mt-6 space-y-4"
          >
            <label className="block">
              <Label>TMDB API key (movies and shows)</Label>
              <input value={tmdb} onChange={(e) => setTmdb(e.target.value)} className={fieldClass} placeholder="Paste your key" spellCheck={false} />
              <KeyLink href="https://www.themoviedb.org/settings/api">Get a TMDB key</KeyLink>
            </label>
            <label className="block">
              <Label>TheTVDB API key (shows, episode orders)</Label>
              <input value={tvdb} onChange={(e) => setTvdb(e.target.value)} className={fieldClass} placeholder="Paste your key" spellCheck={false} />
              <KeyLink href="https://thetvdb.com/api-information">Get a TheTVDB key</KeyLink>
            </label>
            {setup.error && <p className="text-sm text-warning">{setup.error.message}</p>}
            <div className="flex gap-3 pt-1">
              <Button type="button" variant="ghost" size="lg" onClick={() => setStep('account')} disabled={setup.isPending}>
                Back
              </Button>
              <Button type="submit" variant="primary" size="lg" className="flex-1" disabled={setup.isPending}>
                {setup.isPending && <Loader2 className="animate-spin" />}
                {tmdb || tvdb ? 'Finish' : 'Skip for Now'}
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
