import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Copy, Loader2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { inputClass } from '@/components/ui/field';
import { ErrorNotice } from '@/components/Page';

/** The one-time credential stays in component state, never the query/mutation cache or storage. */
export function ToolTokenForm() {
  const { t } = useTranslation('accessSettings');
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [secret, setSecret] = useState('');
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState(false);
  const create = useMutation({
    mutationFn: async () => {
      const created = await api.createToolToken(name.trim());
      setSecret(created.token); setCopied(false); setCopyError(false); setName('');
      await qc.invalidateQueries({ queryKey: ['devices'] });
    },
  });
  const close = () => { setSecret(''); setOpen(false); setCopyError(false); create.reset(); };
  return <div className="space-y-3 p-4">
    {!open ? <Button onClick={() => setOpen(true)}>{t('tools.newToken')}</Button> : <>
      <p className="text-sm text-muted">{t('tools.description')}</p>
      {!secret && <form className="flex flex-wrap items-end gap-3" onSubmit={e => { e.preventDefault(); create.mutate(); }}>
        <label className="min-w-48 flex-1 text-sm">{t('tools.name')}
          <input className={inputClass + ' mt-2'} maxLength={100} required value={name} onChange={e => setName(e.target.value)} autoComplete="off" />
        </label>
        <Button type="submit" variant="primary" disabled={!name.trim() || create.isPending}>
          {create.isPending && <Loader2 className="animate-spin" />}{t('tools.create')}
        </Button>
        <Button type="button" variant="ghost" disabled={create.isPending} onClick={close}>{t('tools.cancel')}</Button>
      </form>}
      {create.error && <ErrorNotice error={create.error} />}
      {secret && <div role="group" aria-label={t('tools.newSecret')} className="space-y-3 rounded-xl bg-surface-2 p-4">
        <p className="text-sm">{t('tools.shownOnce')}</p>
        <input aria-label={t('tools.newSecret')} className={inputClass + ' font-mono'} readOnly value={secret} autoComplete="off" spellCheck={false} />
        <div className="flex gap-2">
          <Button onClick={() => { void (async () => { try { await navigator.clipboard.writeText(secret); setCopied(true); setCopyError(false); } catch { setCopyError(true); } })(); }}><Copy />{copied ? t('tools.copied') : t('tools.copy')}</Button>
          <Button variant="ghost" onClick={close}>{t('tools.done')}</Button>
        </div>
        {copyError && <p role="alert" className="text-sm text-warning">{t('tools.copyFailed')}</p>}
      </div>}
    </>}
  </div>;
}
