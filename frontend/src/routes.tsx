import { createBrowserRouter, Navigate } from 'react-router-dom'
import { useAuth } from '@/hooks/use-auth'
import { AppShell } from '@/components/AppShell'
import LoginPage from '@/pages/LoginPage'
import DashboardPage from '@/pages/DashboardPage'
import CategoryPage from '@/pages/CategoryPage'

function AuthGuard({ children }: { children: React.ReactNode }) {
  const passkey = useAuth((s) => s.passkey)
  if (!passkey) return <Navigate to="/login" replace />
  return <>{children}</>
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: <Navigate to="/dashboard" replace />,
  },
  {
    path: '/dashboard',
    element: (
      <AuthGuard>
        <AppShell />
      </AuthGuard>
    ),
    children: [
      { index: true, element: <DashboardPage /> },
      { path: ':categoryId', element: <CategoryPage /> },
    ],
  },
])
