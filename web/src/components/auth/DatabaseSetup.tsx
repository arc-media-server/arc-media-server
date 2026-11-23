import { useState, type FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api, type DatabaseSetupInput } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { AuthScreen, Card } from './Screen';
import { PostgresFields } from './PostgresFields';

export function DatabaseSetup() {
  const { t } = useTranslation(['auth', 'common']);
  const qc = useQueryClient();
  const [engine, setEngine] = useState<DatabaseSetupInput['engine']>('sqlite');
  const [fields, setFields] = useState({ host: '', port: '5432', database: '', user: '', password: '' });
  const [noTLS, setNoTLS] = useState(false);
  const choices = {
    sqlite: { name: t('auth:database.sqlite'), description: t('auth:database.sqliteDescription') },
    postgres: { name: t('auth:database.postgres'), description: t('auth:database.postgresDescription') },
  };
  const choose = useMutation({
    mutationFn: async () => {
      await api.chooseDatabase(engine === 'sqlite' ? { engine } : { ...fields, engine, port: Number(fields.port), no_tls: noTLS });
      setFields((old) => ({ ...old, password: '' }));
      // The full API starts after the storage choice has committed.
      for (let attempt = 0; attempt < 50; attempt++) {
        const status = await api.authStatus().catch(() => undefined);
        if (status && !status.database_needed) return status;
        await new Promise((resolve) => window.setTimeout(resolve, 200));
      }
      throw new Error(t('auth:database.startTimeout'));
    },
    onSuccess: (status) => qc.setQueryData(['auth'], status),
  });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    choose.mutate();
  };
  return (
    <AuthScreen>
      <Card>
        <h1 className="text-2xl font-bold">{t('auth:database.heading')}</h1>
        <p className="mt-1.5 text-sm text-muted">{t('auth:database.description')}</p>
        <form onSubmit={submit} className="mt-6 space-y-4">
          <fieldset disabled={choose.isPending} className="space-y-3">
            <legend className="sr-only">{t('auth:database.heading')}</legend>
            {(['sqlite', 'postgres'] as const).map((option) => (
              <label key={option} className="flex items-start gap-3 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
                <input className="mt-1" type="radio" name="engine" checked={engine === option} onChange={() => { setEngine(option); choose.reset(); }} />
                <span>
                  <span className="block text-sm font-medium">{choices[option].name}</span>
                  <span className="block text-xs text-muted">{choices[option].description}</span>
                </span>
              </label>
            ))}
            {engine === 'postgres' && <PostgresFields fields={fields} setFields={setFields} noTLS={noTLS} setNoTLS={setNoTLS} />}
          </fieldset>
          {choose.error && <p role="alert" className="text-sm text-warning">{choose.error.message}</p>}
          <Button type="submit" variant="primary" size="lg" className="w-full" disabled={choose.isPending}>
            {choose.isPending ? t('auth:database.connecting') : t('common:actions.continue')}
          </Button>
        </form>
      </Card>
    </AuthScreen>
  );
}
