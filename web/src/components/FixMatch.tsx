import { Trans, useTranslation } from 'react-i18next';
import { useState, type FormEvent, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Check, CheckCircle2, FileVideo, Folder, Loader2, Search, XCircle } from 'lucide-react';
import { api, type Item, type MatchCandidate, type MatchInfo, type MatchStep, type ParsedName, type Provider } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

const providerName: Record<Provider, string> = { tmdb: 'TMDB', tvdb: 'TheTVDB' };

/**
 * Explains how Arc identified a movie or show (what it read from the file names and what it
 * searched for), and lets the user search for and pick the right match.
 */
export function FixMatchDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <FixMatch item={item} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function FixMatch({ item, onDone }: { item: Item; onDone: () => void }) {
  const { t } = useTranslation('metadata');
  const info = useQuery({ queryKey: ['match', item.id], queryFn: () => api.matchInfo(item.id) });
  const unmatched = item.metadata_state === 'unmatched';


  return (
    <DialogContent
      title={unmatched ? item.kind === 'series' ? t('metadata:fixMatch.identifyShow') : t('metadata:fixMatch.identifyMovie') : t("metadata:fixMatch.fixMatch")}
      description={
        unmatched
          ? item.kind === 'series' ? t('metadata:fixMatch.unmatchedShow') : t('metadata:fixMatch.unmatchedMovie')
          : item.kind === 'series' ? t('metadata:fixMatch.wrongShow') : t('metadata:fixMatch.wrongMovie')
      }
      className="w-[min(680px,calc(100vw-32px))]"
    >
      {info.error ? (
        <p className="text-sm text-warning">{info.error.message}</p>
      ) : !info.data ? (
        <div className="h-40 animate-pulse rounded-2xl bg-surface-2" />
      ) : (
        <div className="space-y-7 pb-3">
          <Explanation info={info.data} />
          <Finder item={item} info={info.data} onDone={onDone} />
        </div>
      )}
    </DialogContent>
  );
}

function Explanation({ info }: { info: MatchInfo }) {
  const { t } = useTranslation('metadata');
  const item = info.item;
  const parsed = info.parsed[0];
  const current = item.tmdb_id || item.tvdb_id;
  return (
    <section>
      <SectionTitle>{item.metadata_state === 'unmatched' ? t("metadata:fixMatch.whyItWasnT") : t("metadata:fixMatch.howItWasIdentified")}</SectionTitle>
      <ol className="space-y-2.5">
        {parsed && <ParsedStep parsed={parsed} kind={item.kind} />}
        {info.steps.map((s, i) => (
          <StepRow key={i} step={s} />
        ))}
        {info.steps.length === 0 && (
          <Row icon={current ? <CheckCircle2 className="text-accent" /> : <Search />}>
            {current ? (
              <>{item.tmdb_id && item.tvdb_id ? t('metadata:fixMatch.matchedBoth', { tmdb: item.tmdb_id, tvdb: item.tvdb_id }) : item.tmdb_id ? t('metadata:fixMatch.matchedTmdb', { id: item.tmdb_id }) : t('metadata:fixMatch.matchedTvdb', { id: item.tvdb_id })}</>
            ) : (
              <>{t("metadata:fixMatch.arcHasnTSearched")}</>
            )}
          </Row>
        )}
      </ol>
    </section>
  );
}

export function ParsedStep({ parsed, kind }: { parsed: ParsedName; kind: Item['kind'] }) {
  const { t } = useTranslation('metadata');
  const ids = [
    parsed.tmdb_id && `TMDB ${parsed.tmdb_id}`,
    parsed.tvdb_id && `TheTVDB ${parsed.tvdb_id}`,
    parsed.imdb_id && `IMDb ${parsed.imdb_id}`,
  ].filter(Boolean);
  return (
    <Row icon={parsed.source_kind === 'folder' ? <Folder /> : <FileVideo />}>
      <p>{kind === 'series' ? parsed.source_kind === 'folder' ? t('metadata:fixMatch.readShowFolder') : t('metadata:fixMatch.readShowFile') : parsed.source_kind === 'folder' ? t('metadata:fixMatch.readMovieFolder') : t('metadata:fixMatch.readMovieFile')}</p>
      <p className="mt-1.5 break-all rounded-lg bg-surface-2 px-2.5 py-1.5 font-mono text-xs text-foreground/90">{parsed.source}</p>
      {parsed.recognised && parsed.title ? (
        <div className="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
          <ArrowRight className="size-3.5 text-subtle" />
          <Chip>{parsed.title}</Chip>
          {parsed.year ? <Chip>{parsed.year}</Chip> : <span className="text-subtle">{t("metadata:fixMatch.noYear")}</span>}
          {ids.map((id) => (
            <Chip key={id as string}>{id}</Chip>
          ))}
        </div>
      ) : (
        <p className="mt-1.5 text-xs text-warning">{t("metadata:fixMatch.arcCouldnTRead")}</p>
      )}
      {ids.length > 0 && <p className="mt-1.5 text-xs text-subtle">{t("metadata:fixMatch.theNameIncludesAn")}</p>}
    </Row>
  );
}

export function StepRow({ step }: { step: MatchStep }) {
  const { t } = useTranslation('metadata');
  const provider = providerName[step.provider];
  let text: ReactNode;
  switch (step.kind) {
    case 'chosen': text = t('metadata:fixMatch.chosen', { provider, id: step.picked }); break;
    case 'find': text = t('metadata:fixMatch.findId', { provider, query: step.query }); break;
    case 'lookup': text = t('metadata:fixMatch.lookup', { provider, query: step.query }); break;
    default: text = step.year ? <Trans ns="metadata" i18nKey="fixMatch.searchYear" values={{ provider, query: step.query, year: step.year }} components={{ query: <span className="text-foreground" /> }} /> : <Trans ns="metadata" i18nKey="fixMatch.searchProvider" values={{ provider, query: step.query }} components={{ query: <span className="text-foreground" /> }} />;
  }
  const failed = !!step.error || (step.kind !== 'chosen' && !step.picked);
  return (
    <Row icon={step.picked ? <CheckCircle2 className="text-accent" /> : failed ? <XCircle className="text-subtle" /> : <Search />}>
      <p>{text}</p>
      {step.results.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {step.results.map((r) => (
            <Chip key={r.id} active={r.id === step.picked}>
              {r.id === step.picked && <Check className="size-3" />}
              {r.title}
              {r.year ? ` (${r.year})` : ''}
            </Chip>
          ))}
        </div>
      )}
      {(step.note || step.error) && <p className={cn('mt-1.5 text-xs', step.error ? 'text-warning' : 'text-subtle')}>{step.error ?? step.note}</p>}
    </Row>
  );
}

function Finder({ item, info, onDone }: { item: Item; info: MatchInfo; onDone: () => void }) {
  const { t } = useTranslation('metadata');
  const qc = useQueryClient();
  const available = (item.kind === 'series' ? (['tvdb', 'tmdb'] as Provider[]) : (['tmdb'] as Provider[])).filter((p) => info.providers[p]);
  const [provider, setProvider] = useState<Provider | undefined>(available[0]);
  const parsed = info.parsed[0];
  const [title, setTitle] = useState(item.metadata_state === 'unmatched' ? (parsed?.title ?? item.title) : item.title);
  const [year, setYear] = useState(String((item.metadata_state === 'unmatched' ? parsed?.year : item.year) || ''));
  const [submitted, setSubmitted] = useState<{ provider: Provider; q: string; year?: number } | null>(
    provider && title ? { provider, q: title, year: Number(year) || undefined } : null,
  );

  const results = useQuery({
    queryKey: ['match-search', item.id, submitted],
    enabled: !!submitted,
    queryFn: () => api.searchMatches(item.id, submitted!.provider, submitted!.q, submitted!.year),
  });

  const choose = useMutation({
    mutationFn: (c: MatchCandidate) => api.setMatch(item.id, c.provider, c.id),
    onSuccess: (updated) => {
      qc.setQueryData(['item', item.id], updated);
      qc.invalidateQueries({ queryKey: ['item', item.id] });
      qc.invalidateQueries({ queryKey: ['match', item.id] });
      qc.invalidateQueries({ queryKey: ['seasons', item.id] });
      qc.invalidateQueries({ queryKey: ['orderings', item.id] });
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['health'] });
      qc.invalidateQueries({ queryKey: ['health-summary'] });
      qc.invalidateQueries({ queryKey: ['featured'] });
      onDone();
    },
  });

  const search = (e?: FormEvent, p = provider) => {
    e?.preventDefault();
    if (p && title.trim()) setSubmitted({ provider: p, q: title.trim(), year: Number(year) || undefined });
  };

  if (available.length === 0) {
    return <p className="text-sm text-muted">{item.kind === 'series' ? t('metadata:fixMatch.addSeriesKey') : t('metadata:fixMatch.addMovieKey')}</p>;
  }

  const currentId = (c: MatchCandidate) => (c.provider === 'tmdb' ? item.tmdb_id : item.tvdb_id) === c.id && item.metadata_state === 'matched';

  return (
    <section>
      <SectionTitle>{item.kind === 'series' ? t('metadata:fixMatch.findShow') : t('metadata:fixMatch.findMovie')}</SectionTitle>
      <form onSubmit={search} className="flex flex-wrap gap-2">
        {available.length > 1 && (
          <div className="flex h-10 rounded-xl bg-surface-2/80 p-1 ring-1 ring-border">
            {available.map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => {
                  setProvider(p);
                  search(undefined, p);
                }}
                className={cn(
                  'rounded-lg px-3 text-[13px] font-medium transition-colors',
                  provider === p ? 'bg-surface-3 text-foreground shadow-sm' : 'text-muted hover:text-foreground',
                )}
              >
                {providerName[p]}
              </button>
            ))}
          </div>
        )}
        <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("metadata:fixMatch.title")} className={cn(inputClass, 'min-w-40 flex-1')} />
        <input
          value={year}
          onChange={(e) => setYear(e.target.value.replace(/\D/g, '').slice(0, 4))}
          placeholder={t("metadata:fixMatch.year")}
          inputMode="numeric"
          className={cn(inputClass, 'w-20')}
        />
        <Button type="submit" variant="secondary" disabled={!title.trim()}>
          <Search /> {t("metadata:fixMatch.search")}</Button>
      </form>

      <div className="mt-4 space-y-2">
        {results.isFetching && <div className="h-24 animate-pulse rounded-2xl bg-surface-2" />}
        {results.error && <p className="text-sm text-warning">{results.error.message}</p>}
        {!results.isFetching && results.data?.length === 0 && (
          <p className="py-6 text-center text-sm text-muted">{year ? t('metadata:fixMatch.emptyYear') : t('metadata:fixMatch.empty')}.</p>
        )}
        {!results.isFetching &&
          results.data?.map((c) => (
            <Candidate
              key={`${c.provider}-${c.id}`}
              candidate={c}
              current={currentId(c)}
              busy={choose.isPending && choose.variables?.id === c.id}
              disabled={choose.isPending}
              onChoose={() => choose.mutate(c)}
            />
          ))}
        {choose.error && <p className="text-sm text-warning">{choose.error.message}</p>}
      </div>
    </section>
  );
}

function Candidate({
  candidate: c,
  current,
  busy,
  disabled,
  onChoose,
}: {
  candidate: MatchCandidate;
  current: boolean;
  busy: boolean;
  disabled: boolean;
  onChoose: () => void;
}) {
  const { t } = useTranslation('metadata');
  const [imgFailed, setImgFailed] = useState(false);
  return (
    <div className={cn('flex gap-3.5 rounded-2xl p-2.5 ring-1 transition-colors', current ? 'bg-accent/8 ring-accent/30' : 'bg-surface-2/40 ring-border')}>
      <div className="aspect-[2/3] w-14 shrink-0 overflow-hidden rounded-lg bg-surface-3">
        {c.poster && !imgFailed && (
          <img src={c.poster} alt="" loading="lazy" className="size-full object-cover" onError={() => setImgFailed(true)} />
        )}
      </div>
      <div className="min-w-0 flex-1 py-0.5">
        <p className="truncate text-sm font-semibold">
          {c.title}
          {c.year ? <span className="font-normal text-muted"> ({c.year})</span> : null}
        </p>
        {c.original_title && <p className="truncate text-xs text-subtle">{c.original_title}</p>}
        {c.overview && <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-muted">{c.overview}</p>}
        <p className="mt-1 text-[11px] text-subtle">
          {providerName[c.provider]} · {c.id}
        </p>
      </div>
      <div className="flex shrink-0 items-center">
        {current ? (
          <span className="flex items-center gap-1 px-2 text-xs font-medium text-accent">
            <Check className="size-3.5" /> {t("metadata:fixMatch.current")}</span>
        ) : (
          <Button size="sm" variant="primary" onClick={onChoose} disabled={disabled}>
            {busy && <Loader2 className="animate-spin" />} {t("metadata:fixMatch.useThis")}</Button>
        )}
      </div>
    </div>
  );
}


function SectionTitle({ children }: { children: ReactNode }) {
  return <h3 className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-subtle">{children}</h3>;
}

function Row({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <li className="flex gap-3 text-sm text-muted">
      <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center text-subtle [&_svg]:size-4">{icon}</span>
      <div className="min-w-0 flex-1">{children}</div>
    </li>
  );
}

function Chip({ children, active }: { children: ReactNode; active?: boolean }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs',
        active ? 'bg-accent/15 font-medium text-accent' : 'bg-surface-2 text-foreground/80',
      )}
    >
      {children}
    </span>
  );
}
