import { useState, type ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2, Lock, LockOpen } from 'lucide-react';
import { api, type Item, type ItemEdit, type LockableField } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

type TextField = Exclude<LockableField, 'poster' | 'backdrop'>;

interface FieldSpec {
  field: TextField;
  label: string;
  type?: 'text' | 'number' | 'date' | 'textarea';
  placeholder?: string;
  half?: boolean;
}

function fieldsFor(kind: Item['kind']): FieldSpec[] {
  const dateLabel = kind === 'movie' ? 'Release Date' : kind === 'series' ? 'First Aired' : 'Air Date';
  const all: (FieldSpec | false)[] = [
    { field: 'title', label: 'Title' },
    kind !== 'episode' && { field: 'sort_title', label: 'Sort As', placeholder: 'Same as the title' },
    kind !== 'episode' && { field: 'original_title', label: 'Original Title' },
    kind !== 'episode' && { field: 'year', label: 'Year', type: 'number', half: true },
    { field: 'release_date', label: dateLabel, type: 'date', half: true },
    kind !== 'series' && { field: 'runtime_min', label: 'Runtime (minutes)', type: 'number', half: true },
    kind !== 'episode' && { field: 'content_rating', label: 'Age Rating', half: true, placeholder: 'e.g. 12A' },
    kind !== 'episode' && { field: 'studio', label: kind === 'series' ? 'Network' : 'Studio', half: true },
    kind !== 'episode' && { field: 'tagline', label: 'Tagline' },
    { field: 'overview', label: 'Overview', type: 'textarea' },
  ];
  return all.filter((f): f is FieldSpec => !!f);
}

function initialValue(item: Item, field: TextField): string {
  switch (field) {
    case 'year':
      return item.year ? String(item.year) : '';
    case 'runtime_min':
      return item.runtime_min ? String(item.runtime_min) : '';
    case 'sort_title':
      // Only show a sort title the user chose; otherwise it just follows the title.
      return item.locked_fields.includes('sort_title') ? item.sort_title : '';
    default:
      return item[field] ?? '';
  }
}

/** A dialog to correct an item's details. Edited fields are locked. */
export function EditDetailsDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <EditForm item={item} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function EditForm({ item, onDone }: { item: Item; onDone: () => void }) {
  const qc = useQueryClient();
  const specs = fieldsFor(item.kind);
  const [values, setValues] = useState(() => Object.fromEntries(specs.map((s) => [s.field, initialValue(item, s.field)])) as Record<TextField, string>);
  const [locked, setLocked] = useState(() => new Set<LockableField>(item.locked_fields));
  const provider = item.kind === 'movie' ? 'TMDB' : 'TMDB and TheTVDB';

  const save = useMutation({
    mutationFn: async () => {
      const edit: ItemEdit = { locked_fields: [...locked] };
      for (const s of specs) {
        const v = values[s.field].trim();
        if (v === initialValue(item, s.field).trim()) continue;
        if (s.field === 'year' || s.field === 'runtime_min') edit[s.field] = v ? Number(v) : 0;
        else edit[s.field] = v;
      }
      let updated = await api.editItem(item.id, edit);
      // Fields just unlocked should go back to the online value straight away.
      if (item.locked_fields.some((f) => !locked.has(f))) {
        try {
          updated = await api.refreshItem(item.id);
        } catch {
          // Keep the saved edit; the value updates on the next refresh.
        }
      }
      return updated;
    },
    onSuccess: (updated) => {
      qc.setQueryData(['item', item.id], (old: Item | undefined) => (old ? { ...old, ...updated } : updated));
      qc.invalidateQueries({ queryKey: ['item', item.id] });
      qc.invalidateQueries({ queryKey: ['seasons'] });
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['featured'] });
      onDone();
    },
  });

  const setValue = (field: TextField, v: string) => {
    setValues((prev) => ({ ...prev, [field]: v }));
    setLocked((prev) => new Set(prev).add(field));
  };
  const toggle = (field: LockableField) =>
    setLocked((prev) => {
      const next = new Set(prev);
      if (next.has(field)) next.delete(field);
      else next.add(field);
      return next;
    });

  return (
    <DialogContent
      title="Edit Details"
      description={<>Anything you change is locked, so refreshing from {provider} won't undo it. Unlock a field to use the online value again.</>}
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost" disabled={save.isPending}>
              Cancel
            </Button>
          </DialogClose>
          <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending || !values.title?.trim()}>
            {save.isPending && <Loader2 className="animate-spin" />} Save
          </Button>
        </>
      }
    >
      <form
        className="grid grid-cols-2 gap-x-4 gap-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {specs.map((s) => (
          <Field key={s.field} label={s.label} locked={locked.has(s.field)} onToggle={() => toggle(s.field)} half={s.half}>
            {s.type === 'textarea' ? (
              <textarea
                value={values[s.field]}
                onChange={(e) => setValue(s.field, e.target.value)}
                rows={5}
                className={cn(inputClass, 'h-auto resize-y py-2.5 leading-relaxed')}
              />
            ) : (
              <input
                type={s.type ?? 'text'}
                value={values[s.field]}
                placeholder={s.field === 'sort_title' ? values.title || s.placeholder : s.placeholder}
                min={s.type === 'number' ? 0 : undefined}
                onChange={(e) => setValue(s.field, e.target.value)}
                className={inputClass}
              />
            )}
          </Field>
        ))}
        <div className="col-span-2 mt-1">
          <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">Artwork</p>
          <div className="divide-y divide-border overflow-hidden rounded-xl bg-surface-2/50 ring-1 ring-border">
            <ArtworkLock label={item.kind === 'episode' ? 'Keep this thumbnail' : 'Keep this poster'} locked={locked.has('poster')} onToggle={() => toggle('poster')} />
            {item.kind !== 'episode' && <ArtworkLock label="Keep this background" locked={locked.has('backdrop')} onToggle={() => toggle('backdrop')} />}
          </div>
        </div>
        <button type="submit" hidden />
      </form>
    </DialogContent>
  );
}


function Field({ label, locked, onToggle, half, children }: { label: string; locked: boolean; onToggle: () => void; half?: boolean; children: ReactNode }) {
  return (
    <label className={cn('block', half ? 'col-span-2 sm:col-span-1' : 'col-span-2')}>
      <span className="mb-1.5 flex items-center justify-between">
        <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</span>
        <LockToggle locked={locked} onToggle={onToggle} />
      </span>
      {children}
    </label>
  );
}

function LockToggle({ locked, onToggle }: { locked: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        onToggle();
      }}
      title={locked ? 'Locked: refreshes keep this' : 'Unlocked: updated from the internet on refresh'}
      aria-pressed={locked}
      className={cn(
        'flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium transition-colors',
        locked ? 'bg-accent/15 text-accent' : 'text-subtle hover:bg-surface-3 hover:text-muted',
      )}
    >
      {locked ? <Lock className="size-3" /> : <LockOpen className="size-3" />}
      {locked ? 'Locked' : 'Auto'}
    </button>
  );
}

function ArtworkLock({ label, locked, onToggle }: { label: string; locked: boolean; onToggle: () => void }) {
  return (
    <div className="flex items-center justify-between gap-3 px-3.5 py-2.5">
      <span className="text-sm">{label}</span>
      <LockToggle locked={locked} onToggle={onToggle} />
    </div>
  );
}
