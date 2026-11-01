// The API documentation page (/docs/): Scalar's open-source viewer reading the OpenAPI
// description this server generates from its code (/api/openapi.json). Everything is bundled
// into Arc and nothing is fetched from elsewhere: no web fonts, telemetry or AI helper.
import { createApiReference } from '@scalar/api-reference';
import '@scalar/api-reference/style.css';

createApiReference('#docs', {
  url: '/api/openapi.json',
  // "Try it" requests go to this server, signed in as the current browser session.
  servers: [{ url: `${location.origin}/api/v1`, description: 'This Arc server' }],
  // Arc's colours: Scalar's neutral theme, black and white like the app (web/src/index.css).
  theme: 'default',
  customCss: `.light-mode { --scalar-color-accent: #1d1d1f; --scalar-background-accent: #1d1d1f1f; }
.dark-mode { --scalar-color-accent: #f5f5f7; --scalar-background-accent: #f5f5f71f; }`,
  telemetry: false,
  withDefaultFonts: false,
  agent: { disabled: true },
  mcp: { disabled: true },
  documentDownloadType: 'json',
  // Scalar's own toolbar (Share, Deploy, Generate SDK) links to its hosted services.
  showDeveloperTools: 'never',
  defaultOpenFirstTag: false,
  metaData: { title: 'Arc API' },
});
