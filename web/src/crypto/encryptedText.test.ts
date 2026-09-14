import { describe, expect, it } from "vitest"
import {
  createNewEncryptedText,
  decryptWithFragment,
  encryptWithFragment,
  ENCRYPTED_TEXT_AAD,
  ENCRYPTED_TEXT_PROTOCOL,
  KEY_FRAGMENT_PREFIX,
  MAX_CONTENT_BYTES,
  type EncryptedPayloadV1,
  type TextFormat,
  parseKeyFragment,
} from "./encryptedText"

// Static interoperability vector independently generated with Go crypto/aes + cipher.NewGCM.
const VECTOR_KEY = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8"
const VECTOR_NONCE = "oKGio6Slpqeoqaqr"
const VECTOR_CIPHERTEXT = "5xoJfSS4dtpCFfW8cxWjsRyMLyGyVd7_0GUFs_5Om4VV275yEuMUJQ"
const VECTOR_FRAGMENT = `${KEY_FRAGMENT_PREFIX}${VECTOR_KEY}`

function encode(bytes: Uint8Array): string {
  let binary = ""
  for (let offset = 0; offset < bytes.length; offset += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000))
  }
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "")
}

function decode(value: string): Uint8Array<ArrayBuffer> {
  const binary = atob(value.replaceAll("-", "+").replaceAll("_", "/") + "=".repeat((4 - (value.length % 4)) % 4))
  return Uint8Array.from(binary, (character) => character.charCodeAt(0))
}

async function encryptedEnvelope(envelope: Uint8Array<ArrayBuffer>, nonceByte: number): Promise<EncryptedPayloadV1> {
  const key = await globalThis.crypto.subtle.importKey("raw", decode(VECTOR_KEY), "AES-GCM", false, ["encrypt"])
  const nonce = new Uint8Array(12).fill(nonceByte)
  const ciphertext = new Uint8Array(
    await globalThis.crypto.subtle.encrypt(
      { name: "AES-GCM", iv: nonce, additionalData: new TextEncoder().encode(ENCRYPTED_TEXT_AAD), tagLength: 128 },
      key,
      envelope,
    ),
  )
  return { protocol: ENCRYPTED_TEXT_PROTOCOL, nonce: encode(nonce), ciphertext: encode(ciphertext) }
}

describe("encrypted text protocol", () => {
  it("creates canonical keys, nonces, and protocol payloads", async () => {
    const created = await createNewEncryptedText("PLAIN", "x")
    expect(created.fragment).toMatch(/^#up_e1_[A-Za-z0-9_-]{43}$/)
    expect(decode(created.fragment.slice(KEY_FRAGMENT_PREFIX.length))).toHaveLength(32)
    expect(decode(created.payload.nonce)).toHaveLength(12)
    expect(created.payload.protocol).toBe(ENCRYPTED_TEXT_PROTOCOL)
    await expect(parseKeyFragment(created.fragment)).resolves.toBeTruthy()
  })

  for (const format of ["PLAIN", "SOURCE", "MARKDOWN"] satisfies TextFormat[]) {
    it(`round trips ${format}`, async () => {
      const created = await createNewEncryptedText(format, "content")
      await expect(decryptWithFragment(created.fragment, created.payload)).resolves.toEqual({ format, content: "content" })
    })
  }

  it.each([
    ["Unicode", "é e\u0301 ☃ 🚀"],
    ["whitespace", " \n\t "],
    ["one byte", "x"],
  ])("round trips %s content", async (_name, content) => {
    const created = await createNewEncryptedText("PLAIN", content)
    expect((await decryptWithFragment(created.fragment, created.payload)).content).toBe(content)
  })

  it("enforces content bounds including exactly 1 MiB", async () => {
    await expect(createNewEncryptedText("PLAIN", "")).rejects.toThrow()
    await expect(createNewEncryptedText("PLAIN", "x".repeat(MAX_CONTENT_BYTES + 1))).rejects.toThrow()
    const created = await createNewEncryptedText("PLAIN", "x".repeat(MAX_CONTENT_BYTES))
    expect(decode(created.payload.ciphertext)).toHaveLength(MAX_CONTENT_BYTES + 2 + 16)
    expect((await decryptWithFragment(created.fragment, created.payload)).content).toHaveLength(MAX_CONTENT_BYTES)
  })

  it("uses fresh nonces for new and update encryption", async () => {
    const created = await createNewEncryptedText("PLAIN", "one")
    const updated = await encryptWithFragment(created.fragment, "PLAIN", "two")
    const updatedAgain = await encryptWithFragment(created.fragment, "PLAIN", "three")
    expect(new Set([created.payload.nonce, updated.nonce, updatedAgain.nonce]).size).toBe(3)
  })

  it("rejects wrong keys and modified nonce, ciphertext, or tag", async () => {
    const created = await createNewEncryptedText("PLAIN", "authenticated")
    const wrong = await createNewEncryptedText("PLAIN", "other")
    await expect(decryptWithFragment(wrong.fragment, created.payload)).rejects.toThrow()

    const ciphertext = decode(created.payload.ciphertext)
    const modifiedCiphertext = ciphertext.slice()
    modifiedCiphertext[0] ^= 1
    await expect(decryptWithFragment(created.fragment, { ...created.payload, ciphertext: encode(modifiedCiphertext) })).rejects.toThrow()
    const modifiedTag = ciphertext.slice()
    modifiedTag[modifiedTag.length - 1] ^= 1
    await expect(decryptWithFragment(created.fragment, { ...created.payload, ciphertext: encode(modifiedTag) })).rejects.toThrow()
    const nonce = decode(created.payload.nonce)
    nonce[0] ^= 1
    await expect(decryptWithFragment(created.fragment, { ...created.payload, nonce: encode(nonce) })).rejects.toThrow()
  })

  it("strictly rejects malformed protocol encodings", async () => {
    const created = await createNewEncryptedText("PLAIN", "x")
    for (const fragment of ["", "#up_e2_" + VECTOR_KEY, "up_e1_" + VECTOR_KEY, VECTOR_FRAGMENT + "=", VECTOR_FRAGMENT.slice(0, -1), "#up_e1_" + "A".repeat(42) + "+", "#up_e1_" + "A".repeat(42) + "B"]) {
      await expect(parseKeyFragment(fragment)).rejects.toThrow()
    }
    await expect(decryptWithFragment(created.fragment, { ...created.payload, protocol: "OTHER" as typeof ENCRYPTED_TEXT_PROTOCOL })).rejects.toThrow()
    for (const nonce of ["abc=", "AB", "A".repeat(15), "A".repeat(17)]) {
      await expect(decryptWithFragment(created.fragment, { ...created.payload, nonce })).rejects.toThrow()
    }
    for (const ciphertext of ["abc=", "AB", "+".repeat(26), "A".repeat(25)]) {
      await expect(decryptWithFragment(created.fragment, { ...created.payload, ciphertext })).rejects.toThrow()
    }
  })

  it("rejects authenticated invalid envelopes and malformed UTF-8", async () => {
    await expect(decryptWithFragment(VECTOR_FRAGMENT, await encryptedEnvelope(new Uint8Array([2, 1, 120]), 1))).rejects.toThrow()
    await expect(decryptWithFragment(VECTOR_FRAGMENT, await encryptedEnvelope(new Uint8Array([1, 4, 120]), 2))).rejects.toThrow()
    await expect(decryptWithFragment(VECTOR_FRAGMENT, await encryptedEnvelope(new Uint8Array([1, 1, 0xc3, 0x28]), 3))).rejects.toThrow()
  })

  it("matches the AES-GCM interoperability vector", async () => {
    const payload: EncryptedPayloadV1 = {
      protocol: ENCRYPTED_TEXT_PROTOCOL,
      nonce: VECTOR_NONCE,
      ciphertext: VECTOR_CIPHERTEXT,
    }
    await expect(decryptWithFragment(VECTOR_FRAGMENT, payload)).resolves.toEqual({
      format: "SOURCE",
      content: "uPaste protocol v1 ✓",
    })

    const key = await globalThis.crypto.subtle.importKey("raw", decode(VECTOR_KEY), "AES-GCM", false, ["encrypt"])
    const envelope = new Uint8Array([1, 2, ...new TextEncoder().encode("uPaste protocol v1 ✓")])
    const encrypted = new Uint8Array(
      await globalThis.crypto.subtle.encrypt(
        { name: "AES-GCM", iv: decode(VECTOR_NONCE), additionalData: new TextEncoder().encode(ENCRYPTED_TEXT_AAD), tagLength: 128 },
        key,
        envelope,
      ),
    )
    expect(encode(encrypted)).toBe(VECTOR_CIPHERTEXT)
  })
})
