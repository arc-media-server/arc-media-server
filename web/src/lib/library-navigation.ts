import type { Library } from './api';

/** Sort only libraries already allowed by the server. Unknown IDs grant no access;
 * new libraries keep their server order after the explicitly arranged ones. */
export function navigationLibraries(libraries: Library[], order: number[] = []): Library[] {
  const rank = new Map(order.map((id, index) => [id, index]));
  return [...libraries].sort((a, b) => (rank.get(a.id) ?? order.length) - (rank.get(b.id) ?? order.length));
}
