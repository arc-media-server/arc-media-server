import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Loader2, Lock, Pencil, Plus, Trash2 } from 'lucide-react';
import { api, type Profile, type ProfileColor } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Avatar, profileColors } from '@/components/Avatar';
import { ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';

export function ProfilesPage() {
  const qc = useQueryClient();
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: api.profiles });
  const [editing, setEditing] = useState<Profile | 'new' | null>(null);
  const [removing, setRemoving] = useState<Profile | null>(null);
  const remove = useMutation({
    mutationFn: (p: Profile) => api.deleteProfile(p.id),
    onSuccess: () => {
      setRemoving(null);
      qc.invalidateQueries({ queryKey: ['profiles'] });
      qc.invalidateQueries({ queryKey: ['auth'] });
    },
  });

  if (profiles.error) return <ErrorState error={profiles.error} />;

  return (
    <Page className="pb-16">
      <PageHeader
        title="Profiles"
        subtitle="Everyone in your household gets their own watch history and Continue Watching. A PIN keeps a profile private."
        actions={
          <Button variant="secondary" size="sm" onClick={() => setEditing('new')}>
            <Plus /> Add Profile
          </Button>
        }
      />
      <div className="px-6 md:px-10">
        <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
          {!profiles.data && <div className="h-32 animate-pulse" />}
          {profiles.data?.map((p) => (
            <div key={p.id} className="flex items-center gap-4 px-4 py-3">
              <Avatar name={p.name} color={p.color} className="size-10 text-base" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold">{p.name}</p>
                <p className="flex items-center gap-1.5 text-xs text-subtle">
                  {p.is_admin ? 'Admin · manages libraries and settings' : 'Can watch everything'}
                  {p.has_pin && (
                    <>
                      {' · '}
                      <Lock className="size-3" /> PIN
                    </>
                  )}
                </p>
              </div>
              <Button variant="secondary" size="sm" onClick={() => setEditing(p)}>
                <Pencil /> Edit
              </Button>
              {!p.is_admin && (
                <Button variant="ghost" size="icon" aria-label={`Remove ${p.name}`} title="Remove profile" onClick={() => setRemoving(p)}>
                  <Trash2 />
                </Button>
              )}
            </div>
          ))}
        </div>
      </div>

      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        {editing && <ProfileForm profile={editing === 'new' ? undefined : editing} onDone={() => setEditing(null)} />}
      </Dialog>
      <ConfirmDialog
        open={!!removing}
        onOpenChange={(o) => !o && setRemoving(null)}
        title={`Remove ${removing?.name ?? ''}?`}
        confirmLabel="Remove"
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => removing && remove.mutate(removing)}
      >
        <p>Their watch history and Continue Watching are deleted. This can't be undone, except by restoring a backup.</p>
      </ConfirmDialog>
    </Page>
  );
}

function ProfileForm({ profile, onDone }: { profile?: Profile; onDone: () => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState(profile?.name ?? '');
  const [color, setColor] = useState<ProfileColor>(profile?.color ?? 'blue');
  // PIN: unchanged (undefined), a new PIN, or removed ("").
  const [pin, setPin] = useState<string | undefined>(undefined);
  const [settingPin, setSettingPin] = useState(false);

  const save = useMutation({
    mutationFn: () =>
      profile ? api.updateProfile(profile.id, { name, color, pin }) : api.createProfile({ name, color, pin: pin || undefined }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['profiles'] });
      qc.invalidateQueries({ queryKey: ['auth'] });
      onDone();
    },
  });
  const hasPin = pin === undefined ? !!profile?.has_pin : pin !== '';
  const pinValid = pin === undefined || pin === '' || /^\d{4}$/.test(pin);

  return (
    <DialogContent
      title={profile ? `Edit ${profile.name}` : 'Add a Profile'}
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost">Cancel</Button>
          </DialogClose>
          <Button variant="primary" disabled={!name.trim() || !pinValid || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} Save
          </Button>
        </>
      }
    >
      <form
        className="space-y-6 pb-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim() && pinValid) save.mutate();
        }}
      >
        <div className="flex justify-center">
          <Avatar name={name || '?'} color={color} className="size-24 text-4xl" />
        </div>
        <label className="block">
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Name</span>
          <input autoFocus value={name} onChange={(e) => setName(e.target.value)} maxLength={40} className={inputClass} />
        </label>
        <div>
          <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Colour</span>
          <div className="flex flex-wrap gap-2.5">
            {(Object.keys(profileColors) as ProfileColor[]).map((c) => (
              <button
                key={c}
                type="button"
                aria-label={c}
                aria-pressed={color === c}
                onClick={() => setColor(c)}
                className={cn('flex size-9 items-center justify-center rounded-full bg-gradient-to-br ring-offset-2 ring-offset-surface transition-all', profileColors[c], color === c && 'ring-2 ring-foreground')}
              >
                {color === c && <Check className="size-4 text-white" />}
              </button>
            ))}
          </div>
        </div>
        <div>
          <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">PIN</span>
          {settingPin ? (
            <div className="flex items-center gap-2">
              <input
                autoFocus
                value={pin ?? ''}
                onChange={(e) => setPin(e.target.value.replace(/\D/g, '').slice(0, 4))}
                inputMode="numeric"
                placeholder="4 digits"
                className={cn(inputClass, 'w-32 text-center tracking-[0.4em]')}
              />
              <Button type="button" variant="ghost" size="sm" onClick={() => (setSettingPin(false), setPin(undefined))}>
                Cancel
              </Button>
            </div>
          ) : (
            <div className="flex items-center gap-3">
              <p className="flex-1 text-sm text-muted">
                {hasPin ? 'A PIN is needed to use this profile.' : 'Anyone can use this profile.'}
                {profile?.is_admin && !hasPin && ' At home, the admin profile asks for the password instead.'}
              </p>
              {hasPin && (
                <Button type="button" variant="ghost" size="sm" onClick={() => setPin('')}>
                  Remove PIN
                </Button>
              )}
              <Button type="button" variant="secondary" size="sm" onClick={() => (setSettingPin(true), setPin(''))}>
                {hasPin ? 'Change PIN' : 'Add PIN'}
              </Button>
            </div>
          )}
        </div>
        <button type="submit" hidden />
      </form>
    </DialogContent>
  );
}

const inputClass = 'h-10 w-full rounded-xl bg-surface-2/80 px-3 text-sm ring-1 ring-border outline-none focus:ring-2 focus:ring-accent';
