import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

export const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap font-medium select-none transition-all duration-200 ease-smooth active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        // Bright, high-contrast call to action (Play).
        primary: 'bg-foreground text-background hover:bg-foreground/90 shadow-lg shadow-black/20',
        accent: 'bg-accent-strong text-accent-foreground hover:bg-accent-strong/85',
        secondary: 'bg-surface-2/80 text-foreground hover:bg-surface-3 backdrop-blur-xl',
        ghost: 'text-muted hover:text-foreground hover:bg-surface-2',
        glass: 'bg-white/12 text-white hover:bg-white/20 backdrop-blur-xl',
        // Confirms something that can't be undone.
        danger: 'bg-warning text-black hover:bg-warning/90',
      },
      size: {
        sm: 'h-8 rounded-lg px-3 text-[13px]',
        md: 'h-10 rounded-xl px-4 text-sm',
        lg: 'h-12 rounded-2xl px-6 text-[15px] [&_svg]:size-5',
        icon: 'size-9 rounded-full',
      },
    },
    defaultVariants: { variant: 'secondary', size: 'md' },
  },
);

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(({ className, variant, size, ...props }, ref) => (
  <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
));
Button.displayName = 'Button';
