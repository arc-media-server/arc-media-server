// Shared appearance for the endpoint reference and bundled client guide.
import '@scalar/api-reference/style.css';

export const documentationAppearance = {
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
} as const;
