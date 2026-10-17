import { useEffect, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { motion } from 'motion/react';
import { ChevronLeft, Loader2, Lock, LogOut } from 'lucide-react';
import { api, ApiError, type AuthStatus, type Profile } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Avatar } from '@/components/Avatar';
import { Button } from '@/components/ui/button';
import { AuthScreen, fieldClass } from './Screen';

/** "Who's watching?": pick a profile, entering its PIN (or for the admin at home, the password). */
export function ProfilePicker({ status }: { status: AuthStatus }) {
  const qc = useQueryClient();
  const [chosen, setChosen] = useState<Profile | null>(null);
  const [needs, setNeeds] = useState<'pin' | 'password' | null>(null);

  const choose = useMutation({
    mutationFn: ({ profile, secret }: { profile: Profile; secret?: { pin?: string; password?: string } }) => api.chooseProfile(profile.id, secret),
    onSuccess: () => qc.invalidateQueries(),
    onError: (err, { profile }) => {
      if (err instanceof ApiError && (err.needs === 'pin' || err.needs === 'password')) {
        setChosen(profile);
        setNeeds(err.needs);
      }
    },
  });

  const pick = (p: Profile) => {
    choose.reset();
    if (p.has_pin) {
      setChosen(p);
      setNeeds('pin');
    } else {
      choose.mutate({ profile: p });
    }
  };
  const back = () => {
    setChosen(null);
    setNeeds(null);
    choose.reset();
  };

  if (chosen && needs) {
    const wrong = choose.error instanceof ApiError && choose.error.status !== 401 ? choose.error.message : choose.error && 'Try again';
    return (
      <AuthScreen>
        <div className="flex flex-col items-center text-center">
          <Avatar name={chosen.name} color={chosen.color} className="size-24 text-4xl" />
          <h1 className="mt-4 text-2xl font-semibold">{chosen.name}</h1>
          <p className="mt-1 text-sm text-muted">{needs === 'pin' ? 'Enter your PIN' : 'Enter the password to use the admin profile'}</p>
          <div className="mt-6 w-full max-w-xs">
            {needs === 'pin' ? (
              <PinEntry key={choose.submittedAt} error={!!choose.error} busy={choose.isPending} onComplete={(pin) => choose.mutate({ profile: chosen, secret: { pin } })} />
            ) : (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  const password = new FormData(e.currentTarget).get('password') as string;
                  choose.mutate({ profile: chosen, secret: { password } });
                }}
                className="space-y-3"
              >
                <input name="password" type="password" autoFocus className={fieldClass} autoComplete="current-password" />
                <Button type="submit" variant="primary" size="lg" className="w-full" disabled={choose.isPending}>
                  {choose.isPending && <Loader2 className="animate-spin" />} Continue
                </Button>
              </form>
            )}
            {wrong && <p className="mt-3 text-sm text-warning">{wrong}</p>}
          </div>
          <Button variant="ghost" className="mt-8" onClick={back}>
            <ChevronLeft /> Profiles
          </Button>
        </div>
      </AuthScreen>
    );
  }

  return (
    <AuthScreen wide>
      <h1 className="text-center text-3xl font-bold">Who's watching?</h1>
      <div className="mt-10 flex flex-wrap justify-center gap-6 sm:gap-8">
        {status.profiles.map((p, i) => (
          <motion.button
            key={p.id}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.05 * i, duration: 0.35 }}
            onClick={() => pick(p)}
            disabled={choose.isPending}
            className="group flex w-28 flex-col items-center gap-3 outline-none"
          >
            <span className="relative rounded-full ring-0 ring-foreground transition-all duration-200 group-hover:ring-4 group-focus-visible:ring-4">
              <Avatar name={p.name} color={p.color} className="size-24 text-4xl sm:size-28" />
              {(p.has_pin || (p.is_admin && !status.password_session)) && (
                <span className="absolute -bottom-0.5 -right-0.5 flex size-7 items-center justify-center rounded-full bg-surface-3 text-muted ring-4 ring-background">
                  <Lock className="size-3.5" />
                </span>
              )}
            </span>
            <span className="truncate text-sm font-medium text-muted transition-colors group-hover:text-foreground">{p.name}</span>
          </motion.button>
        ))}
      </div>
      {choose.error && !needs && <p className="mt-6 text-center text-sm text-warning">{choose.error.message}</p>}
      {status.password_session && (
        <div className="mt-12 flex justify-center">
          <Button
            variant="ghost"
            size="sm"
            onClick={async () => {
              await api.logout();
              qc.invalidateQueries();
            }}
          >
            <LogOut /> Sign Out
          </Button>
        </div>
      )}
    </AuthScreen>
  );
}

/** Four boxes for a PIN; calls onComplete when all four digits are in. */
function PinEntry({ onComplete, error, busy }: { onComplete: (pin: string) => void; error: boolean; busy: boolean }) {
  const [digits, setDigits] = useState('');
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    input.current?.focus();
  }, []);
  return (
    <motion.label
      animate={error ? { x: [0, -10, 10, -6, 6, 0] } : undefined}
      transition={{ duration: 0.35 }}
      className="relative flex cursor-text justify-center gap-3"
    >
      <input
        ref={input}
        value={digits}
        inputMode="numeric"
        autoComplete="off"
        maxLength={4}
        disabled={busy}
        aria-label="PIN"
        onChange={(e) => {
          const v = e.target.value.replace(/\D/g, '').slice(0, 4);
          setDigits(v);
          if (v.length === 4) onComplete(v);
        }}
        className="absolute inset-0 opacity-0"
      />
      {[0, 1, 2, 3].map((i) => (
        <span
          key={i}
          className={cn(
            'flex size-14 items-center justify-center rounded-2xl bg-surface-2 text-2xl ring-1 transition-colors',
            i === digits.length ? 'ring-2 ring-accent' : 'ring-border',
            error && 'ring-warning',
          )}
        >
          {digits[i] ? '•' : ''}
        </span>
      ))}
    </motion.label>
  );
}
