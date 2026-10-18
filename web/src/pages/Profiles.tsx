import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Copy, Link2, Loader2, Lock, Pencil, Plus, SlidersHorizontal, Trash2, UserPlus } from 'lucide-react';
import { api, type Profile, type ProfileColor } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Avatar, profileColors } from '@/components/Avatar';
import { ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';
import { Switch } from '@/components/ui/switch';
import { useLibraries } from '@/components/Sidebar';
import { ChipInput } from '@/components/ChipInput';
import { EditOtherHomeDialog, type HomeTarget } from '@/components/EditHome';

export function ProfilesPage() {
  const qc = useQueryClient();
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: api.profiles });
  const [editing, setEditing] = useState<Profile | 'new' | 'friend' | null>(null);
  // A friend's invite link to show.
  const [link, setLink] = useState<{ name: string; token: string } | null>(null);
  const reinvite = useMutation({
    mutationFn: (p: Profile) => api.reinvite(p.id),
    onSuccess: (r, p) => setLink({ name: p.name, token: r.invite_token }),
  });
  const [removing, setRemoving] = useState<Profile | null>(null);
  const [home, setHome] = useState<HomeTarget | null>(null);
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
          <>
            <Button variant="ghost" size="sm" onClick={() => setHome({ kind: 'default' })} title="The home page for profiles that haven't arranged their own">
              <SlidersHorizontal /> Default Home
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setEditing('friend')} title="Share libraries with someone outside your household">
              <UserPlus /> Add Friend
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setEditing('new')}>
              <Plus /> Add Profile
            </Button>
          </>
        }
      />
      <div className="px-6 md:px-10">
        <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
          {!profiles.data && <div className="h-32 animate-pulse" />}
          {profiles.data?.filter((p) => !p.is_friend).map((p) => (
            <div key={p.id} className="flex items-center gap-4 px-4 py-3">
              <Avatar name={p.name} color={p.color} className="size-10 text-base" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold">{p.name}</p>
                <p className="flex items-center gap-1.5 text-xs text-subtle">
                  {p.is_admin
                    ? 'Admin · manages libraries and settings'
                    : [limitLabel(p.max_age), p.library_ids && libraryCount(p.library_ids.length), p.exclude_labels?.length && `hides ${p.exclude_labels.join(', ')}`]
                        .filter(Boolean)
                        .join(' · ')}
                  {p.has_pin && (
                    <>
                      {' · '}
                      <Lock className="size-3" /> PIN
                    </>
                  )}
                </p>
              </div>
              {!p.is_admin && (
                <Button variant="ghost" size="sm" onClick={() => setHome({ kind: 'profile', id: p.id, name: p.name })}>
                  <SlidersHorizontal /> Home
                </Button>
              )}
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
        <h2 className="mb-1 mt-10 text-lg font-semibold">Friends</h2>
        <p className="mb-4 text-sm text-muted">
          People outside your household. They sign in from anywhere with their own name and password, only see the libraries you share, and never see your profiles.
        </p>
        <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
          {profiles.data && !profiles.data.some((p) => p.is_friend) && (
            <p className="px-4 py-3.5 text-sm text-muted">No friends yet. Add Friend makes an invite link to send them.</p>
          )}
          {profiles.data?.filter((p) => p.is_friend).map((p) => (
            <div key={p.id} className="flex items-center gap-4 px-4 py-3">
              <Avatar name={p.name} color={p.color} className="size-10 text-base" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold">{p.name}</p>
                <p className="truncate text-xs text-subtle">
                  {[p.joined ? 'Joined' : "Invited, hasn't joined yet", p.library_ids ? libraryCount(p.library_ids.length) : 'All libraries', p.max_age > 0 && limitLabel(p.max_age)]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                disabled={reinvite.isPending}
                onClick={() => reinvite.mutate(p)}
                title={p.joined ? 'A new link lets them choose a new password' : 'A new invite link (the old one stops working)'}
              >
                <Link2 /> {p.joined ? 'Reset Password' : 'Invite Link'}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setEditing(p)}>
                <Pencil /> Edit
              </Button>
              <Button variant="ghost" size="icon" aria-label={`Remove ${p.name}`} title="Remove friend" onClick={() => setRemoving(p)}>
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
      </div>

      <InviteLinkDialog link={link} onOpenChange={(o) => !o && setLink(null)} />
      <EditOtherHomeDialog target={home} open={!!home} onOpenChange={(o) => !o && setHome(null)} />
      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        {editing && (
          <ProfileForm
            profile={typeof editing === 'string' ? undefined : editing}
            friend={editing === 'friend' || (typeof editing === 'object' && !!editing.is_friend)}
            onDone={(p) => {
              setEditing(null);
              if (p?.invite_token) setLink({ name: p.name, token: p.invite_token });
            }}
          />
        )}
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

/** A friend's invite link, to copy and send them. */
function InviteLinkDialog({ link, onOpenChange }: { link: { name: string; token: string } | null; onOpenChange: (o: boolean) => void }) {
  const [copied, setCopied] = useState(false);
  const url = link ? `${window.location.origin}/invite/${link.token}` : '';
  return (
    <Dialog open={!!link} onOpenChange={(o) => (setCopied(false), onOpenChange(o))}>
      {link && (
        <DialogContent
          title={`Invite ${link.name}`}
          description="Send them this link. With it they choose a password, then sign in with their name and password. It works once, for 7 days."
          footer={
            <DialogClose asChild>
              <Button variant="primary">Done</Button>
            </DialogClose>
          }
        >
          <div className="flex items-center gap-2 pb-2">
            <input readOnly value={url} onFocus={(e) => e.target.select()} className={cn(inputClass, 'flex-1 font-mono text-xs')} />
            <Button
              variant="secondary"
              onClick={() => {
                navigator.clipboard?.writeText(url).then(() => setCopied(true), () => {});
              }}
            >
              {copied ? <Check /> : <Copy />} {copied ? 'Copied' : 'Copy'}
            </Button>
          </div>
          {/^https?:\/\/(localhost|127\.|192\.168\.|10\.)/.test(url) && (
            <p className="pb-2 text-xs text-subtle">This link uses your home address. For a friend elsewhere, open Arc at its outside address (see Remote Access in Settings) and make the link there.</p>
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}

function ProfileForm({ profile, friend, onDone }: { profile?: Profile; friend?: boolean; onDone: (p?: Profile) => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState(profile?.name ?? '');
  const [color, setColor] = useState<ProfileColor>(profile?.color ?? 'blue');
  // PIN: unchanged (undefined), a new PIN, or removed ("").
  const [pin, setPin] = useState<string | undefined>(undefined);
  const [settingPin, setSettingPin] = useState(false);
  const [maxAge, setMaxAge] = useState(profile?.max_age ?? 0);
  // null for every library.
  const [libraryIds, setLibraryIds] = useState<number[] | null>(profile?.library_ids ?? null);
  const [hiddenLabels, setHiddenLabels] = useState<string[]>(profile?.exclude_labels ?? []);
  const allLabels = useQuery({ queryKey: ['labels'], queryFn: api.labels });
  const libraries = useLibraries().data ?? [];

  const save = useMutation({
    mutationFn: () =>
      profile
        ? api.updateProfile(profile.id, {
            name,
            color,
            pin,
            max_age: profile.is_admin ? undefined : maxAge,
            library_ids: profile.is_admin ? undefined : libraryIds,
            exclude_labels: profile.is_admin ? undefined : hiddenLabels,
          })
        : api.createProfile({ name, color, friend, pin: pin || undefined, max_age: maxAge, library_ids: libraryIds, exclude_labels: hiddenLabels }),
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['profiles'] });
      qc.invalidateQueries({ queryKey: ['auth'] });
      onDone(p);
    },
  });
  const hasPin = pin === undefined ? !!profile?.has_pin : pin !== '';
  const pinValid = pin === undefined || pin === '' || /^\d{4}$/.test(pin);

  return (
    <DialogContent
      title={profile ? `Edit ${profile.name}` : friend ? 'Add a Friend' : 'Add a Profile'}
      description={!profile && friend ? "Choose what they can watch. Next you'll get an invite link to send them." : undefined}
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
        {!friend && (
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
        )}
        {!profile?.is_admin && (
          <div>
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Can watch</span>
            <div className="grid grid-cols-2 gap-2">
              {ageLimits.map((l) => (
                <button
                  key={l.age}
                  type="button"
                  aria-pressed={maxAge === l.age}
                  onClick={() => setMaxAge(l.age)}
                  className={cn(
                    'rounded-xl px-3 py-2.5 text-left ring-1 transition-colors',
                    maxAge === l.age ? 'bg-accent/10 ring-2 ring-accent' : 'bg-surface-2/50 ring-border hover:bg-surface-2',
                  )}
                >
                  <span className="block text-sm font-medium">{l.label}</span>
                  <span className="block text-xs text-subtle">{l.hint}</span>
                </button>
              ))}
            </div>
            {maxAge > 0 && <p className="mt-2 text-xs text-subtle">Titles without an age rating are hidden too. You can add a rating with Edit on its page.</p>}
          </div>
        )}
        {!profile?.is_admin && libraries.length > 1 && (
          <div>
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Libraries</span>
            <div className="divide-y divide-border overflow-hidden rounded-xl bg-surface-2/50 ring-1 ring-border">
              <label className="flex cursor-pointer items-center gap-3 px-3 py-2.5">
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium">All libraries</span>
                  <span className="block text-xs text-subtle">Including ones you add later</span>
                </span>
                <Switch checked={libraryIds === null} onCheckedChange={(all) => setLibraryIds(all ? null : libraries.map((l) => l.id))} />
              </label>
              {libraryIds !== null &&
                libraries.map((l) => (
                  <label key={l.id} className="flex cursor-pointer items-center gap-3 px-3 py-2.5 text-sm">
                    <input
                      type="checkbox"
                      checked={libraryIds.includes(l.id)}
                      onChange={(e) => setLibraryIds((ids) => (e.target.checked ? [...(ids ?? []), l.id] : (ids ?? []).filter((x) => x !== l.id)))}
                      className="size-4 accent-[var(--color-accent)]"
                    />
                    {l.name}
                  </label>
                ))}
            </div>
            {libraryIds?.length === 0 && <p className="mt-2 text-xs text-warning">This profile won't see anything until you choose a library.</p>}
          </div>
        )}
        {!profile?.is_admin && (
          <div>
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Hide titles labelled</span>
            <ChipInput value={hiddenLabels} onChange={setHiddenLabels} placeholder="e.g. Not for kids" morePlaceholder="Add a label" suggestions={allLabels.data} />
            <p className="mt-2 text-xs text-subtle">Add labels to titles with Edit on their page.</p>
          </div>
        )}
        <button type="submit" hidden />
      </form>
    </DialogContent>
  );
}

// Age limits a profile can have, like Plex's restriction profiles.
const ageLimits = [
  { age: 0, label: 'Everything', hint: 'No restrictions' },
  { age: 7, label: 'Young children', hint: 'Rated for ages up to 7' },
  { age: 12, label: 'Older children', hint: 'Rated for ages up to 12' },
  { age: 15, label: 'Teens', hint: 'Rated for ages up to 15' },
];

/** "Can watch everything" or the profile's limit, for the list. */
function limitLabel(age: number) {
  return age ? `Ages up to ${age}` : 'Can watch everything';
}


function libraryCount(n: number) {
  return n === 0 ? 'No libraries' : n === 1 ? '1 library' : `${n} libraries`;
}
