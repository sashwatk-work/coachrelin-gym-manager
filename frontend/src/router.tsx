import { lazy, Suspense } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { RequireAuth, RedirectIfAuthed, RequireRole } from '@/lib/auth'
import { AppShell } from '@/components/layout/AppShell'
import { Loading } from '@/components/ui'

const LoginPage = lazy(() => import('@/pages/LoginPage'))
const DashboardPage = lazy(() => import('@/pages/DashboardPage'))
const MembersPage = lazy(() => import('@/pages/MembersPage'))
const MemberDetailPage = lazy(() => import('@/pages/MemberDetailPage'))
const TrainersPage = lazy(() => import('@/pages/TrainersPage'))
const TrainerDetailPage = lazy(() => import('@/pages/TrainerDetailPage'))
const PlansPage = lazy(() => import('@/pages/PlansPage'))
const TrainingPage = lazy(() => import('@/pages/TrainingPage'))
const PaymentsPage = lazy(() => import('@/pages/PaymentsPage'))
const SettingsPage = lazy(() => import('@/pages/SettingsPage'))

function page(el: React.ReactNode) {
  return <Suspense fallback={<Loading />}>{el}</Suspense>
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <RedirectIfAuthed>{page(<LoginPage />)}</RedirectIfAuthed>,
  },
  {
    path: '/register',
    element: <Navigate to="/login" replace />,
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppShell />
      </RequireAuth>
    ),
    children: [
      { index: true, element: page(<DashboardPage />) },
      { path: 'members', element: page(<MembersPage />) },
      { path: 'members/:id', element: page(<MemberDetailPage />) },
      { path: 'trainers', element: page(<TrainersPage />) },
      { path: 'trainers/:id', element: page(<TrainerDetailPage />) },
      { path: 'plans', element: <RequireRole roles={['owner', 'receptionist']}>{page(<PlansPage />)}</RequireRole> },
      { path: 'training', element: page(<TrainingPage />) },
      { path: 'payments', element: <RequireRole roles={['owner', 'receptionist']}>{page(<PaymentsPage />)}</RequireRole> },
      { path: 'settings', element: page(<SettingsPage />) },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])
