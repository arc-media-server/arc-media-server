// The API documentation page (/docs/): Scalar's open-source viewer reading the OpenAPI
// description this server generates from its code (/api/openapi.json). Everything is bundled
// into Arc and nothing is fetched from elsewhere: no web fonts, telemetry or AI helper.
import { createApiReference } from '@scalar/api-reference';
import { documentationAppearance } from './appearance';

createApiReference('#docs', {
  ...documentationAppearance,
  url: '/api/openapi.json',
  // "Try it" requests go to this server, signed in as the current browser session.
  servers: [{ url: `${location.origin}/api/v1`, description: 'This Arc server' }],
  metaData: { title: 'Arc API' },
});
