// Countries with their own age ratings, most likely first after sorting by name.
const regionCodes = ['AU', 'BR', 'CA', 'DE', 'DK', 'ES', 'FI', 'FR', 'GB', 'IE', 'IN', 'IT', 'JP', 'KR', 'MX', 'NL', 'NO', 'NZ', 'PL', 'PT', 'SE', 'US', 'ZA'];
const regionNames = new Intl.DisplayNames(['en'], { type: 'region' });

/** [code, name] pairs sorted by name, including the current region even if it isn't listed. */
export function regionOptions(current?: string): [string, string][] {
  const codes = current && !regionCodes.includes(current) ? [...regionCodes, current] : regionCodes;
  return codes.map((c) => [c, regionNames.of(c) ?? c] as [string, string]).sort((a, b) => a[1].localeCompare(b[1]));
}
