import { useEffect, useRef, useState } from 'react'
import { Button, type ButtonVariant } from './Button'

export interface CopyButtonProps {
  textToCopy: string
  label?: string
  copiedLabel?: string
  variant?: ButtonVariant
  className?: string
  onCopied?: () => void
  onCopyFailed?: (text: string) => void
  ariaLabel?: string
}

export function CopyButton({
  textToCopy,
  label = 'Copy',
  copiedLabel = 'Copied',
  variant = 'secondary',
  className = '',
  onCopied,
  onCopyFailed,
  ariaLabel,
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (timerRef.current) {
        window.clearTimeout(timerRef.current)
      }
    }
  }, [])

  const handleCopy = async () => {
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(textToCopy)
      } else {
        // Fallback for older browsers or insecure contexts
        const textarea = document.createElement('textarea')
        textarea.value = textToCopy
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        try {
          textarea.select()
          if (!document.execCommand('copy')) throw new Error('Copy failed')
        } finally {
          textarea.remove()
        }
      }
    } catch {
      setCopied(false)
      if (timerRef.current) window.clearTimeout(timerRef.current)
      if (onCopyFailed) onCopyFailed(textToCopy)
      else prompt('Copy to clipboard: Ctrl+C, Enter', textToCopy)
      return
    }

    setCopied(true)
    onCopied?.()
    if (timerRef.current) window.clearTimeout(timerRef.current)
    timerRef.current = window.setTimeout(() => setCopied(false), 1500)
  }

  return (
    <Button
      type="button"
      variant={variant}
      className={`btn-copy ${className}`}
      onClick={handleCopy}
      aria-label={copied ? copiedLabel : ariaLabel ?? `${label} to clipboard`}
    >
      <span aria-live="polite">{copied ? copiedLabel : label}</span>
    </Button>
  )
}
