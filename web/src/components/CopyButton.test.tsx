import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { CopyButton } from './CopyButton'

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard')
const originalExecCommand = Object.getOwnPropertyDescriptor(document, 'execCommand')

function setClipboard(value: unknown) {
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value })
}

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard)
  else Reflect.deleteProperty(navigator, 'clipboard')
  if (originalExecCommand) Object.defineProperty(document, 'execCommand', originalExecCommand)
  else Reflect.deleteProperty(document, 'execCommand')
  vi.restoreAllMocks()
})

describe('CopyButton failure handling', () => {
  it('reports denied clipboard access without a false Copied state or prompt', async () => {
    const writeText = vi.fn().mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error('denied'))
    const onCopyFailed = vi.fn()
    const promptSpy = vi.spyOn(window, 'prompt').mockImplementation(() => null)
    setClipboard({ writeText })
    render(<CopyButton textToCopy="up_o1_secret" label="Copy token" onCopyFailed={onCopyFailed} />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy token to clipboard' }))
    await screen.findByRole('button', { name: 'Copied' })
    fireEvent.click(screen.getByRole('button', { name: 'Copied' }))

    await waitFor(() => expect(onCopyFailed).toHaveBeenCalledWith('up_o1_secret'))
    expect(screen.getByRole('button', { name: 'Copy token to clipboard' })).toBeInTheDocument()
    expect(promptSpy).not.toHaveBeenCalled()
  })

  it('treats a legacy copy false result as failure and removes the temporary textarea', async () => {
    setClipboard(undefined)
    Object.defineProperty(document, 'execCommand', { configurable: true, value: vi.fn().mockReturnValue(false) })
    const onCopyFailed = vi.fn()
    render(<CopyButton textToCopy="up_o1_secret" onCopyFailed={onCopyFailed} />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy to clipboard' }))

    await waitFor(() => expect(onCopyFailed).toHaveBeenCalledWith('up_o1_secret'))
    expect(screen.getByRole('button', { name: 'Copy to clipboard' })).toBeInTheDocument()
    expect(document.querySelector('textarea')).toBeNull()
  })

  it('removes the temporary textarea when legacy copy throws', async () => {
    setClipboard(undefined)
    Object.defineProperty(document, 'execCommand', { configurable: true, value: vi.fn().mockImplementation(() => { throw new Error('denied') }) })
    const onCopyFailed = vi.fn()
    render(<CopyButton textToCopy="up_o1_secret" onCopyFailed={onCopyFailed} />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy to clipboard' }))

    await waitFor(() => expect(onCopyFailed).toHaveBeenCalledWith('up_o1_secret'))
    expect(document.querySelector('textarea')).toBeNull()
  })
})
