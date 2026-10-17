import type { ProfileColor } from '@/lib/api';
import { cn } from '@/lib/utils';

export const profileColors: Record<ProfileColor, string> = {
  violet: 'from-violet-400 to-indigo-600',
  blue: 'from-sky-400 to-blue-600',
  teal: 'from-teal-300 to-cyan-600',
  green: 'from-emerald-400 to-green-700',
  amber: 'from-amber-300 to-yellow-600',
  orange: 'from-orange-400 to-red-600',
  rose: 'from-pink-400 to-rose-600',
  slate: 'from-slate-400 to-slate-700',
};

/** A profile's picture: its initial on its colour. */
export function Avatar({ name, color, className }: { name: string; color: ProfileColor | string; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        'flex shrink-0 select-none items-center justify-center rounded-full bg-gradient-to-br font-semibold text-white shadow-inner',
        profileColors[color as ProfileColor] ?? profileColors.violet,
        className,
      )}
    >
      {name.trim().charAt(0).toUpperCase() || '?'}
    </span>
  );
}
