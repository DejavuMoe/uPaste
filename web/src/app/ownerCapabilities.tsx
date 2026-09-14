import { createContext, useContext, useRef, type ReactNode } from 'react'

export interface OwnerCapabilitiesContextType {
  remember: (shareID: string, ownerToken: string) => void
  get: (shareID: string) => string | undefined
  forget: (shareID: string) => void
  clear: () => void
}

const OwnerCapabilitiesContext = createContext<OwnerCapabilitiesContextType | null>(null)

export function OwnerCapabilityProvider({ children }: { children: ReactNode }) {
  // Purely in-memory Map living only within the active React application memory.
  // Never serialized to history.state, localStorage, sessionStorage, or cookies.
  const storeRef = useRef<Map<string, string>>(new Map())

  const remember = (shareID: string, ownerToken: string) => {
    storeRef.current.set(shareID, ownerToken)
  }

  const get = (shareID: string) => {
    return storeRef.current.get(shareID)
  }

  const forget = (shareID: string) => {
    storeRef.current.delete(shareID)
  }

  const clear = () => {
    storeRef.current.clear()
  }

  return (
    <OwnerCapabilitiesContext.Provider value={{ remember, get, forget, clear }}>
      {children}
    </OwnerCapabilitiesContext.Provider>
  )
}

export function useOwnerCapabilities(): OwnerCapabilitiesContextType {
  const ctx = useContext(OwnerCapabilitiesContext)
  if (!ctx) {
    throw new Error('useOwnerCapabilities must be used within an OwnerCapabilityProvider')
  }
  return ctx
}
