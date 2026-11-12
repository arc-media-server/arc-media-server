// Render the canonical Markdown through Scalar's already bundled Markdown renderer.
// The core guide works offline; specialist repository links are opened only on click.
import { createApiReference } from '@scalar/api-reference';
import { documentationAppearance } from './appearance';
import guide from '../../../docs/CLIENT-INTEGRATION.md?raw';

const title = guide.match(/^# (.+)$/m)?.[1] ?? 'Arc client integration';
const repositoryDocs = 'https://github.com/arc-media-server/arc-media-server/blob/main/docs/';
const description = guide.replace(/^# .+\r?\n/, '').replace(/\]\(([^)]+)\)/g, (link, target: string) => {
  if (target.startsWith('#') || /^[a-z]+:/i.test(target) || target.startsWith('/')) return link;
  if (target.split('#')[0] === 'API.md') return '](/docs/)';
  if (target === 'openapi.json') return '](/api/openapi.json)';
  return `](${new URL(target, repositoryDocs).href})`;
});

createApiReference('#docs', {
  ...documentationAppearance,
  content: { openapi: '3.1.0', info: { title, version: '1', description }, paths: {} },
  showSidebar: false,
  hideModels: true,
  hideClientButton: true,
  hiddenClients: true, // No generated request snippets for a prose-only walkthrough.
  hideDownloadButton: true,
  metaData: { title },
});
