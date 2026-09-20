import clsx from 'clsx'
import type { ButtonHTMLAttributes } from 'react'
import { buttonClass, type ButtonSize, type ButtonVariant } from '@/utils/buttonStyles'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
  size?: ButtonSize
}

/** 基础按钮（样式表见 `utils/buttonStyles.ts`，`<Link>` 可用 `buttonClass()` 复用） */
export default function Button({ variant, size, className, ...rest }: Props) {
  return <button className={clsx(buttonClass({ variant, size }), className)} {...rest} />
}
