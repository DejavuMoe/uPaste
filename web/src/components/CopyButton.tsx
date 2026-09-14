import { useEffect, useRef, useState } from 'react'
import { Button, type ButtonVariant } from './Button'

export interface CopyButtonProps {
  textToCopy: string
  label?: string
  copiedLabel?: string
  variant?: ButtonVariant
  className?: string
  onCopied?: () => void
}

export function CopyButton({
  textToCopy,
  label = 'Copy',
  copiedLabel = 'Copied',
  variant = 'secondary',
  className = '',
  onCopied,
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
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }

      setCopied(true)
      onCopied?.()

      if (timerRef.current) {
        window.clearTimeout(timerRef.current)
      }
      timerRef.current = window.setTimeout(() => {
        setCopied(false)
      }, 1500)
    } catch {
      // If copy fails completely, trigger fallback selection or prompt
      prompt('Copy to clipboard: Ctrl+C, Enter', textToCopy)
    }
  }

  return (
    <Button
      type="button"
      variant={variant}
      className={`btn-copy ${className}`}
      onClick={handleCopy}
      aria-label={copied ? copiedLabel : `${label} to clipboard`}
    >
      <span aria-live="polite">{copied ? copiedLabel : label}</span>
    </Button>
  )
}
