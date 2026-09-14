export type PayloadKind = 'TEXT' | 'FILE'
export type PrivacyMode = 'STANDARD' | 'ENCRYPTED'
export type TextFormat = 'PLAIN' | 'SOURCE' | 'MARKDOWN'

export interface CreateStandardTextRequest {
  payload_kind: 'TEXT'
  privacy_mode: 'STANDARD'
  text: {
    format: TextFormat
    content: string
  }
  expires_at: string | null
}

export interface CreateEncryptedTextRequest {
  payload_kind: 'TEXT'
  privacy_mode: 'ENCRYPTED'
  encrypted_text: {
    protocol: 'UPASTE_AES_GCM_V1'
    nonce: string
    ciphertext: string
  }
  expires_at: string | null
}

export interface CreateFileMetadata {
  payload_kind: 'FILE'
  privacy_mode: 'STANDARD'
  expires_at: string | null
}

export interface ShareMetadata {
  id: string
  payload_kind: PayloadKind
  privacy_mode: PrivacyMode
  created_at: string
  updated_at: string
  expires_at: string | null
  text?: {
    format: TextFormat
    content: string
  }
  encrypted_text?: {
    protocol: string
    nonce: string
    ciphertext: string
  }
  file?: {
    filename: string
    size: number
    media_type: string
    sha256: string
    download_url: string
  }
}

export interface CreateShareResponse {
  share: ShareMetadata
  owner_token: string
}

export interface GetShareResponse {
  share: ShareMetadata
}

export type SharePatch = {
  text?: { format: TextFormat; content: string }
  encrypted_text?: { protocol: 'UPASTE_AES_GCM_V1'; nonce: string; ciphertext: string }
  expires_at?: string | null
}

export interface UpdateShareResponse {
  share: ShareMetadata
}

export interface ApiErrorResponse {
  error: {
    code: string
    message: string
  }
}

export interface UploadProgress {
  loaded: number
  total: number
  lengthComputable: boolean
  percent: number | null
}

export type Theme = 'system' | 'light' | 'dark'
