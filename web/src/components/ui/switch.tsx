import type { ComponentPropsWithoutRef } from 'react';
import { Switch as S } from 'radix-ui';
import { cn } from '@/lib/utils';

/** An on/off toggle. */
export function Switch({ className, ...props }: ComponentPropsWithoutRef<typeof S.Root>) {
  return (
    <S.Root
      className={cn(
        'relative h-6 w-10 shrink-0 cursor-pointer rounded-full bg-surface-3 transition-colors duration-200 data-[state=checked]:bg-accent-strong disabled:cursor-default disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <S.Thumb className="block size-5 translate-x-0.5 rounded-full bg-white shadow-md transition-transform duration-200 ease-smooth data-[state=checked]:translate-x-[18px]" />
    </S.Root>
  );
}
