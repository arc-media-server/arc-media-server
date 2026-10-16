import type { ComponentPropsWithoutRef, ReactNode } from 'react';
import { DropdownMenu as Menu } from 'radix-ui';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';

export const DropdownMenu = Menu.Root;
export const DropdownMenuTrigger = Menu.Trigger;
export const DropdownMenuRadioGroup = Menu.RadioGroup;

export function DropdownMenuContent({ className, sideOffset = 8, ...props }: ComponentPropsWithoutRef<typeof Menu.Content>) {
  return (
    <Menu.Portal>
      <Menu.Content
        sideOffset={sideOffset}
        className={cn(
          'glass z-50 min-w-56 overflow-hidden rounded-2xl border border-border p-1.5 shadow-2xl shadow-black/40',
          'animate-menu-in origin-(--radix-dropdown-menu-content-transform-origin)',
          'max-h-[min(70vh,var(--radix-dropdown-menu-content-available-height))] overflow-y-auto',
          className,
        )}
        {...props}
      />
    </Menu.Portal>
  );
}

const itemClass =
  'relative flex cursor-default select-none items-center gap-3 rounded-lg px-2.5 py-2 text-sm outline-none transition-colors data-[highlighted]:bg-surface-3 data-[disabled]:opacity-50';

export function DropdownMenuItem({ className, ...props }: ComponentPropsWithoutRef<typeof Menu.Item>) {
  return <Menu.Item className={cn(itemClass, className)} {...props} />;
}

export function DropdownMenuRadioItem({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<typeof Menu.RadioItem> & { children: ReactNode }) {
  return (
    <Menu.RadioItem className={cn(itemClass, 'pr-8', className)} {...props}>
      {children}
      <Menu.ItemIndicator className="absolute right-2.5 text-accent">
        <Check className="size-4" />
      </Menu.ItemIndicator>
    </Menu.RadioItem>
  );
}

export function DropdownMenuLabel({ className, ...props }: ComponentPropsWithoutRef<typeof Menu.Label>) {
  return (
    <Menu.Label
      className={cn('px-2.5 pb-1 pt-2.5 text-[11px] font-semibold uppercase tracking-wider text-subtle', className)}
      {...props}
    />
  );
}

export function DropdownMenuSeparator({ className, ...props }: ComponentPropsWithoutRef<typeof Menu.Separator>) {
  return <Menu.Separator className={cn('mx-1 my-1.5 h-px bg-border', className)} {...props} />;
}
