export const ENCRYPTED_TEXT_PROTOCOL = "UPASTE_AES_GCM_V1" as const
export const ENCRYPTED_TEXT_AAD = "uPaste:encrypted-text:v1"
export const KEY_FRAGMENT_PREFIX = "#up_e1_"
export const MAX_CONTENT_BYTES = 1_048_576
export const MAX_CIPHERTEXT_BYTES = 1_048_594

export type TextFormat = "PLAIN" | "SOURCE" | "MARKDOWN"

export type EncryptedPayloadV1 = {
  protocol: typeof ENCRYPTED_TEXT_PROTOCOL
  nonce: string
  ciphertext: string
}

export type EncryptedText = {
  format: TextFormat
  content: string
}

const formatBytes: Record<TextFormat, number> = { PLAIN: 1, SOURCE: 2, MARKDOWN: 3 }
const formats: Record<number, TextFormat> = { 1: "PLAIN", 2: "SOURCE", 3: "MARKDOWN" }
const encoder = new TextEncoder()
const decoder = new TextDecoder("utf-8", { fatal: true })
const additionalData = encoder.encode(ENCRYPTED_TEXT_AAD)

export async function createNewEncryptedText(format: TextFormat, content: string): Promise<{
  payload: EncryptedPayloadV1
  fragment: string
}> {
  const keyBytes = globalThis.crypto.getRandomValues(new Uint8Array(32))
  const fragment = KEY_FRAGMENT_PREFIX + encodeBase64URL(keyBytes)
  const key = await importKey(keyBytes)
  return { payload: await encrypt(key, format, content), fragment }
}

export async function encryptWithFragment(
  fragment: string,
  format: TextFormat,
  content: string,
): Promise<EncryptedPayloadV1> {
  return encrypt(await parseKeyFragment(fragment), format, content)
}

export async function decryptWithFragment(
  fragment: string,
  payload: EncryptedPayloadV1,
): Promise<EncryptedText> {
  if (payload.protocol !== ENCRYPTED_TEXT_PROTOCOL) throw new Error("unsupported encrypted text protocol")
  const nonce = decodeBase64URL(payload.nonce, 12, 12)
  const ciphertext = decodeBase64URL(payload.ciphertext, 19, MAX_CIPHERTEXT_BYTES)
  const plaintext = new Uint8Array(
    await globalThis.crypto.subtle.decrypt(
      { name: "AES-GCM", iv: nonce, additionalData, tagLength: 128 },
      await parseKeyFragment(fragment),
      ciphertext,
    ),
  )
  if (plaintext.length < 3 || plaintext.length > MAX_CONTENT_BYTES + 2 || plaintext[0] !== 1) {
    throw new Error("invalid encrypted text envelope")
  }
  const format = formats[plaintext[1]]
  if (!format) throw new Error("invalid encrypted text format")
  const contentBytes = plaintext.subarray(2)
  if (contentBytes.length < 1 || contentBytes.length > MAX_CONTENT_BYTES) {
    throw new Error("invalid encrypted text content length")
  }
  return { format, content: decoder.decode(contentBytes) }
}

export async function parseKeyFragment(fragment: string): Promise<CryptoKey> {
  if (!fragment.startsWith(KEY_FRAGMENT_PREFIX) || fragment.length !== KEY_FRAGMENT_PREFIX.length + 43) {
    throw new Error("invalid encrypted text key fragment")
  }
  return importKey(decodeBase64URL(fragment.slice(KEY_FRAGMENT_PREFIX.length), 32, 32))
}

async function encrypt(key: CryptoKey, format: TextFormat, content: string): Promise<EncryptedPayloadV1> {
  const formatByte = formatBytes[format]
  if (!formatByte) throw new Error("invalid encrypted text format")
  const contentBytes = encoder.encode(content)
  if (contentBytes.length < 1 || contentBytes.length > MAX_CONTENT_BYTES) {
    throw new Error("encrypted text content must be between 1 byte and 1 MiB")
  }
  const envelope = new Uint8Array(contentBytes.length + 2)
  envelope[0] = 1
  envelope[1] = formatByte
  envelope.set(contentBytes, 2)
  const nonce = globalThis.crypto.getRandomValues(new Uint8Array(12))
  const ciphertext = new Uint8Array(
    await globalThis.crypto.subtle.encrypt(
      { name: "AES-GCM", iv: nonce, additionalData, tagLength: 128 },
      key,
      envelope,
    ),
  )
  return {
    protocol: ENCRYPTED_TEXT_PROTOCOL,
    nonce: encodeBase64URL(nonce),
    ciphertext: encodeBase64URL(ciphertext),
  }
}

function importKey(bytes: Uint8Array<ArrayBuffer>): Promise<CryptoKey> {
  return globalThis.crypto.subtle.importKey("raw", bytes, { name: "AES-GCM" }, false, ["encrypt", "decrypt"])
}

function encodeBase64URL(bytes: Uint8Array): string {
  let binary = ""
  const chunkSize = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize))
  }
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "")
}

function decodeBase64URL(value: string, minimum: number, maximum: number): Uint8Array<ArrayBuffer> {
  if (
    value.length < Math.ceil((minimum * 4) / 3) ||
    value.length > Math.ceil((maximum * 4) / 3) ||
    !/^[A-Za-z0-9_-]+$/.test(value) ||
    value.length % 4 === 1
  )
    throw new Error("invalid base64url")
  const padded = value.replaceAll("-", "+").replaceAll("_", "/") + "=".repeat((4 - (value.length % 4)) % 4)
  let binary: string
  try {
    binary = atob(padded)
  } catch {
    throw new Error("invalid base64url")
  }
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0))
  if (bytes.length < minimum || bytes.length > maximum || encodeBase64URL(bytes) !== value) {
    throw new Error("invalid base64url")
  }
  return bytes
}
