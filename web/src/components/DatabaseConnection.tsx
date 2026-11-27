import { useState, type FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api, type PostgresConnection } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { PostgresFields, type PostgresFieldsValue } from '@/components/auth/PostgresFields';

export function DatabaseConnection({ connection, canEdit, restartRequired }: {
  connection?: PostgresConnection;
  canEdit: boolean;
  restartRequired: boolean;
}) {
  const { t } = useTranslation('administration');
  return (
    <section aria-label={t('backups.postgresConnection')} className="space-y-3 rounded-2xl bg-surface p-4 text-sm ring-1 ring-border">
      <h2 className="font-medium">{t('backups.postgresConnection')}</h2>
      {restartRequired && <p role="status">{t('backups.restartForConnection')}</p>}
      {connection ? (
        <ConnectionForm connection={connection} canEdit={canEdit} />
      ) : <p className="text-muted">{t('backups.manageConnectionOffline')}</p>}
    </section>
  );
}

function ConnectionForm({ connection, canEdit }: { connection: PostgresConnection; canEdit: boolean }) {
  const { t } = useTranslation('administration');
  const qc = useQueryClient();
  const [fields, setFields] = useState<PostgresFieldsValue>({ host: connection.host, database: connection.database, user: connection.user, port: String(connection.port), password: '' });
  const [noTLS, setNoTLS] = useState(connection.no_tls);
  const update = useMutation({
    mutationFn: () => api.updateDatabaseConnection({ ...fields, port: Number(fields.port), no_tls: noTLS }),
    onSuccess: () => {
      setFields((current) => ({ ...current, password: '' }));
      return qc.invalidateQueries({ queryKey: ['database-settings'] });
    },
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    update.mutate();
  };
  return (
    <form onSubmit={submit} className="max-w-xl space-y-3">
      <p className="text-muted">{canEdit ? t('backups.editConnectionDescription') : t('backups.protectConnection')}</p>
      <fieldset disabled={!canEdit || update.isPending} className="space-y-3">
        <PostgresFields fields={fields} setFields={setFields} noTLS={noTLS} setNoTLS={setNoTLS} />
      </fieldset>
      {update.error && <p role="alert" className="text-warning">{update.error.message}</p>}
      {update.isSuccess && <p role="status">{t('backups.connectionVerified')}</p>}
      <Button type="submit" variant="secondary" size="sm" disabled={!canEdit || update.isPending}>
        {update.isPending ? t('backups.checkingConnection') : t('backups.saveConnection')}
      </Button>
    </form>
  );
}
