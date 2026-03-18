import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/hooks/use-auth'
import * as api from '@/lib/api'

export default function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const setPasskey = useAuth((s) => s.setPasskey)
  const [key, setKey] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async () => {
    if (!key.trim()) return
    setLoading(true)
    setError('')
    try {
      // Temporarily set passkey for the auth interceptor
      localStorage.setItem('passkey', key.trim())
      await api.getCategories()
      // Only persist to Zustand after successful validation
      setPasskey(key.trim())
      navigate('/dashboard')
    } catch {
      localStorage.removeItem('passkey')
      setError(t('login.invalidPasskey'))
    }
    setLoading(false)
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-xs space-y-4">
        <div className="text-center space-y-1">
          <h1 className="text-lg font-semibold">{t('login.title')}</h1>
          <p className="text-sm text-[var(--muted-foreground)]">{t('login.subtitle')}</p>
        </div>
        <div className="space-y-2.5">
          <Input
            type="password"
            placeholder={t('login.placeholder')}
            value={key}
            onChange={(e) => setKey(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleLogin()}
            className="h-9"
            autoFocus
            aria-label={t('login.placeholder')}
          />
          {error && (
            <p className="text-xs text-[var(--danger)] text-center" role="alert">
              {error}
            </p>
          )}
          <Button className="w-full h-9" onClick={handleLogin} loading={loading} disabled={!key.trim()}>
            {t('login.button')}
          </Button>
        </div>
      </div>
    </main>
  )
}
