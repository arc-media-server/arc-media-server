import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation('administration');
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
        title={t("administration:profiles.profiles")}
        subtitle={t("administration:profiles.everyoneInYourHousehold")}
        actions={
          <>
            <Button variant="ghost" size="sm" onClick={() => setHome({ kind: 'default' })} title={t("administration:profiles.theHomePageFor")}>
              <SlidersHorizontal /> {t("administration:profiles.defaultHome")}</Button>
            <Button variant="ghost" size="sm" onClick={() => setEditing('friend')} title={t("administration:profiles.shareLibrariesWithSomeone")}>
              <UserPlus /> {t("administration:profiles.addFriend")}</Button>
            <Button variant="secondary" size="sm" onClick={() => setEditing('new')}>
              <Plus /> {t("administration:profiles.addProfile")}</Button>
          </>
        }
      />
      <div className="px-6 md:px-10">
        <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
          {!profiles.data && <div className="h-32 animate-pulse" />}
          {profiles.data?.filter((p) => !p.is_friend).map((p) => (
            <div key={p.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <Avatar name={p.name} color={p.color} className="size-10 shrink-0 text-base" />
              <div className="min-w-0 basis-[calc(100%-4rem)] sm:basis-0 flex-1">
                <p className="break-words text-sm font-semibold">{p.name}</p>
                <p className="flex flex-wrap items-center gap-1.5 text-xs text-subtle">
                  {p.is_admin
                    ? t("administration:profiles.adminManagesLibrariesAnd")
                    : [limitLabel(p.max_age), p.library_ids && libraryCount(p.library_ids.length), p.exclude_labels?.length && t('administration:profiles.hiddenLabels', { labels: p.exclude_labels.join(', ') })]
                        .filter(Boolean)
                        .join(' · ')}
                  {p.has_pin && (
                    <>
                      {' · '}
                      <Lock className="size-3" /> {t("administration:profiles.pin")}</>
                  )}
                </p>
              </div>
              {!p.is_admin && (
                <Button variant="ghost" size="sm" onClick={() => setHome({ kind: 'profile', id: p.id, name: p.name })}>
                  <SlidersHorizontal /> {t("administration:profiles.home")}</Button>
              )}
              <Button variant="secondary" size="sm" onClick={() => setEditing(p)}>
                <Pencil /> {t("administration:profiles.edit")}</Button>
              {!p.is_admin && (
                <Button variant="ghost" size="icon" aria-label={t('administration:profiles.removeName', { name: p.name })} title={t("administration:profiles.removeProfile")} onClick={() => setRemoving(p)}>
                  <Trash2 />
                </Button>
              )}
            </div>
          ))}
        </div>
        <h2 className="mb-1 mt-10 text-lg font-semibold">{t("administration:profiles.friends")}</h2>
        <p className="mb-4 text-sm text-muted">{t("administration:profiles.peopleOutsideYourHousehold")}</p>
        <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
          {profiles.data && !profiles.data.some((p) => p.is_friend) && (
            <p className="px-4 py-3.5 text-sm text-muted">{t("administration:profiles.noFriendsYetAdd")}</p>
          )}
          {profiles.data?.filter((p) => p.is_friend).map((p) => (
            <div key={p.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <Avatar name={p.name} color={p.color} className="size-10 shrink-0 text-base" />
              <div className="min-w-0 basis-[calc(100%-4rem)] sm:basis-0 flex-1">
                <p className="break-words text-sm font-semibold">{p.name}</p>
                <p className="break-words text-xs text-subtle">
                  {[p.joined ? t('administration:profiles.joined') : t('administration:profiles.invited'), p.library_ids ? libraryCount(p.library_ids.length) : t('administration:profiles.allLibraries'), p.max_age > 0 && limitLabel(p.max_age)]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                disabled={reinvite.isPending}
                onClick={() => reinvite.mutate(p)}
                title={p.joined ? t("administration:profiles.aNewLinkLets") : t("administration:profiles.aNewInviteLink")}
              >
                <Link2 /> {p.joined ? t("administration:profiles.resetPassword") : t("administration:profiles.inviteLink")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setEditing(p)}>
                <Pencil /> {t("administration:profiles.edit")}</Button>
              <Button variant="ghost" size="icon" aria-label={t('administration:profiles.removeName', { name: p.name })} title={t("administration:profiles.removeFriend")} onClick={() => setRemoving(p)}>
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
        title={t('administration:profiles.removeTitle', { name: removing?.name ?? '' })}
        confirmLabel={t("administration:profiles.remove")}
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => removing && remove.mutate(removing)}
      >
        <p>{t("administration:profiles.theirWatchHistoryAnd")}</p>
      </ConfirmDialog>
    </Page>
  );
}

/** A friend's invite link, to copy and send them. */
function InviteLinkDialog({ link, onOpenChange }: { link: { name: string; token: string } | null; onOpenChange: (o: boolean) => void }) {
  const { t } = useTranslation('administration');
  const [copied, setCopied] = useState(false);
  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: api.settings,
    enabled: !!link,
    staleTime: 0,
  });
  const url = link && settings.data
    ? `${settings.data.public_url || window.location.origin}/invite/${link.token}`
    : '';
  return (
    <Dialog open={!!link} onOpenChange={(o) => (setCopied(false), onOpenChange(o))}>
      {link && (
        <DialogContent
          title={t('administration:profiles.inviteTitle', { name: link.name })}
          description={t("administration:profiles.sendThemThisLink")}
          footer={
            <DialogClose asChild>
              <Button variant="primary">{t("administration:profiles.done")}</Button>
            </DialogClose>
          }
        >
          {settings.isFetching ? (
            <Loader2 role="status" aria-label={t('profiles.loadingInviteLink')} className="mx-auto my-4 animate-spin" />
          ) : settings.isError ? (
            <ErrorState error={settings.error} onRetry={() => { void settings.refetch(); }} />
          ) : url && (
            <>
              <div className="flex items-center gap-2 pb-2">
                <input readOnly value={url} onFocus={(e) => e.target.select()} className={cn(inputClass, 'flex-1 font-mono text-xs')} />
                <Button
                  variant="secondary"
                  onClick={() => {
                    navigator.clipboard?.writeText(url).then(() => setCopied(true), () => {});
                  }}
                >
                  {copied ? <Check /> : <Copy />} {copied ? t("administration:profiles.copied") : t("administration:profiles.copy")}
                </Button>
              </div>
              {/^https?:\/\/(localhost|127\.|192\.168\.|10\.)/.test(url) && (
                <p className="pb-2 text-xs text-subtle">{t("administration:profiles.thisLinkUsesYour")}</p>
              )}
            </>
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}

function ProfileForm({ profile, friend, onDone }: { profile?: Profile; friend?: boolean; onDone: (p?: Profile) => void }) {
  const { t } = useTranslation('administration');
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
      title={profile ? t('administration:profiles.editTitle', { name: profile.name }) : friend ? t("administration:profiles.addAFriend") : t("administration:profiles.addAProfile")}
      description={!profile && friend ? t("administration:profiles.chooseWhatTheyCan") : undefined}
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost">{t("administration:profiles.cancel")}</Button>
          </DialogClose>
          <Button variant="primary" disabled={!name.trim() || !pinValid || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} {t("administration:profiles.save")}</Button>
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
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("administration:profiles.name")}</span>
          <input autoFocus value={name} onChange={(e) => setName(e.target.value)} maxLength={40} className={inputClass} />
        </label>
        <div>
          <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("administration:profiles.colour")}</span>
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
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("administration:profiles.pin")}</span>
            {settingPin ? (
              <div className="flex items-center gap-2">
                <input
                  autoFocus
                  value={pin ?? ''}
                  onChange={(e) => setPin(e.target.value.replace(/\D/g, '').slice(0, 4))}
                  inputMode="numeric"
                  placeholder={t("administration:profiles.label4Digits")}
                  className={cn(inputClass, 'w-32 text-center tracking-[0.4em]')}
                />
                <Button type="button" variant="ghost" size="sm" onClick={() => (setSettingPin(false), setPin(undefined))}>{t("administration:profiles.cancel")}</Button>
              </div>
            ) : (
              <div className="flex items-center gap-3">
                <p className="flex-1 text-sm text-muted">
                  {hasPin ? t("administration:profiles.aPinIsNeeded") : profile?.is_admin ? t('administration:profiles.adminWithoutPin') : t("administration:profiles.anyoneCanUseThis")}
                </p>
                {hasPin && (
                  <Button type="button" variant="ghost" size="sm" onClick={() => setPin('')}>{t("administration:profiles.removePin")}</Button>
                )}
                <Button type="button" variant="secondary" size="sm" onClick={() => (setSettingPin(true), setPin(''))}>
                  {hasPin ? t("administration:profiles.changePin") : t("administration:profiles.addPin")}
                </Button>
              </div>
            )}
          </div>
        )}
        {!profile?.is_admin && (
          <div>
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("administration:profiles.canWatch")}</span>
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
            {maxAge > 0 && <p className="mt-2 text-xs text-subtle">{t("administration:profiles.titlesWithoutAnAge")}</p>}
          </div>
        )}
        {!profile?.is_admin && libraries.length > 1 && (
          <div>
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("administration:profiles.libraries")}</span>
            <div className="divide-y divide-border overflow-hidden rounded-xl bg-surface-2/50 ring-1 ring-border">
              <label className="flex cursor-pointer items-center gap-3 px-3 py-2.5">
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium">{t("administration:profiles.allLibraries")}</span>
                  <span className="block text-xs text-subtle">{t("administration:profiles.includingOnesYouAdd")}</span>
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
            {libraryIds?.length === 0 && <p className="mt-2 text-xs text-warning">{t("administration:profiles.thisProfileWonT")}</p>}
          </div>
        )}
        {!profile?.is_admin && (
          <div>
            <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("administration:profiles.hideTitlesLabelled")}</span>
            <ChipInput value={hiddenLabels} onChange={setHiddenLabels} placeholder={t("administration:profiles.eGNotFor")} morePlaceholder={t("administration:profiles.addALabel")} suggestions={allLabels.data} />
            <p className="mt-2 text-xs text-subtle">{t("administration:profiles.addLabelsToTitles")}</p>
          </div>
        )}
        <button type="submit" hidden />
      </form>
    </DialogContent>
  );
}

// Age limits a profile can have, like Plex's restriction profiles.
const ageLimits = [
  { age: 0, get label() { return i18n.t('administration:ageLimits.everything'); }, get hint() { return i18n.t('administration:ageLimits.noRestrictions'); } },
  { age: 7, get label() { return i18n.t('administration:ageLimits.youngChildren'); }, get hint() { return i18n.t('administration:ageLimits.ageSeven'); } },
  { age: 12, get label() { return i18n.t('administration:ageLimits.olderChildren'); }, get hint() { return i18n.t('administration:ageLimits.ageTwelve'); } },
  { age: 15, get label() { return i18n.t('administration:ageLimits.teens'); }, get hint() { return i18n.t('administration:ageLimits.ageFifteen'); } },
];

/** "Can watch everything" or the profile's limit, for the list. */
function limitLabel(age: number) {
  return age ? i18n.t('administration:profiles.maxAge', { age }) : i18n.t('administration:profiles.allAges');
}


function libraryCount(n: number) {
  return i18n.t('administration:profiles.libraryCount', { count: n });
}
