import { useId, useState } from 'react';
import { X } from 'lucide-react';
import { inputClass } from '@/components/ui/field';
import { cn } from '@/lib/utils';

/** Words as removable chips, with a box to type more (Enter or comma adds one). */
export function ChipInput({
  value,
  onChange,
  placeholder,
  morePlaceholder,
  suggestions,
}: {
  value: string[];
  onChange: (value: string[]) => void;
  placeholder: string;
  morePlaceholder: string;
  /** Offered as you type (e.g. labels already in use). */
  suggestions?: string[];
}) {
  const [text, setText] = useState('');
  const listId = useId();
  const add = (raw = text) => {
    const g = raw.trim();
    if (g && !value.some((v) => v.toLowerCase() === g.toLowerCase())) onChange([...value, g]);
    setText('');
  };
  const offered = suggestions?.filter((s) => !value.some((v) => v.toLowerCase() === s.toLowerCase()));
  return (
    <div className={cn(inputClass, 'flex h-auto min-h-10 flex-wrap items-center gap-1.5 py-1.5')}>
      {value.map((g) => (
        <span key={g} className="flex items-center gap-1 rounded-full bg-surface-3 py-0.5 pl-2.5 pr-1 text-[13px]">
          {g}
          <button type="button" aria-label={`Remove ${g}`} onClick={() => onChange(value.filter((v) => v !== g))} className="rounded-full p-0.5 text-subtle hover:text-foreground">
            <X className="size-3" />
          </button>
        </span>
      ))}
      <input
        value={text}
        list={offered?.length ? listId : undefined}
        onChange={(e) => {
          // Picking a suggestion adds it straight away.
          if (offered?.includes(e.target.value)) add(e.target.value);
          else setText(e.target.value);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault();
            add();
          } else if (e.key === 'Backspace' && !text && value.length) {
            onChange(value.slice(0, -1));
          }
        }}
        onBlur={() => add()}
        placeholder={value.length ? morePlaceholder : placeholder}
        className="min-w-24 flex-1 bg-transparent text-sm outline-none placeholder:text-subtle"
      />
      {!!offered?.length && (
        <datalist id={listId}>
          {offered.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
      )}
    </div>
  );
}
