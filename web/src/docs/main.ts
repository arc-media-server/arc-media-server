// The API documentation page (/docs/): Scalar's open-source viewer reading the OpenAPI
// description this server generates from its code (/api/openapi.json). Everything is bundled
// into Arc and nothing is fetched from elsewhere: no web fonts, telemetry or AI helper.
import { createApiReference } from '@scalar/api-reference';
import '@scalar/api-reference/style.css';

createApiReference('#docs', {
  url: '/api/openapi.json',
  // "Try it" requests go to this server, signed in as the current browser session.
  servers: [{ url: `${location.origin}/api/v1`, description: 'This Arc server' }],
  // Arc's colours: Scalar's neutral theme with Arc's teal accent (web/src/index.css).
  theme: 'default',
  customCss: `.light-mode { --scalar-color-accent: #0b7d72; --scalar-background-accent: #0b7d721f; }
.dark-mode { --scalar-color-accent: #2bb3a3; --scalar-background-accent: #2bb3a31f; }`,
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
