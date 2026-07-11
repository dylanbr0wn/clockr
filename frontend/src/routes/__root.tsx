import { createRootRoute, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { AppShell } from '@/components/app-shell'

const RootLayout = () => (
  <AppShell>
    <Outlet />
    <TanStackRouterDevtools />
  </AppShell>
)

export const Route = createRootRoute({ component: RootLayout })
