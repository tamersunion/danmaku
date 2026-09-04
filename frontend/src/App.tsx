import { lazy, Suspense } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  BrowserRouter,
  Navigate,
  Outlet,
  Route,
  Routes,
} from "react-router-dom";

import { useSession } from "@/auth/session-context";
import { AuthBoundary } from "@/auth/session";
import { AppShell } from "@/components/app-shell";
import { ThemeProvider } from "@/components/theme-provider";
import { Spinner } from "@/components/ui/spinner";
import { Toaster } from "@/components/ui/toast";
import { TooltipProvider } from "@/components/ui/tooltip";

const DanmakuPage = lazy(() =>
  import("@/pages/danmaku-page").then((module) => ({
    default: module.DanmakuPage,
  })),
);
const LoginPage = lazy(() =>
  import("@/pages/login-page").then((module) => ({
    default: module.LoginPage,
  })),
);
const ProfilePage = lazy(() =>
  import("@/pages/profile-page").then((module) => ({
    default: module.ProfilePage,
  })),
);
const UsersPage = lazy(() =>
  import("@/pages/users-page").then((module) => ({
    default: module.UsersPage,
  })),
);

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 20_000, retry: 1 } },
});

function HomeRedirect() {
  const session = useSession();
  return (
    <Navigate
      to={session.canManageDanmaku ? "/danmaku" : "/profile"}
      replace
    />
  );
}

function DanmakuManagerRoute() {
  return useSession().canManageDanmaku ? (
    <Outlet />
  ) : (
    <Navigate to="/profile" replace />
  );
}

function UserManagerRoute() {
  return useSession().canManageUsers ? (
    <Outlet />
  ) : (
    <Navigate to="/danmaku" replace />
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <TooltipProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <Suspense
              fallback={
                <main className="grid min-h-svh place-items-center">
                  <Spinner className="size-6" />
                </main>
              }
            >
              <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route element={<AuthBoundary />}>
                  <Route element={<AppShell />}>
                    <Route index element={<HomeRedirect />} />
                    <Route element={<DanmakuManagerRoute />}>
                      <Route path="danmaku" element={<DanmakuPage />} />
                    </Route>
                    <Route element={<UserManagerRoute />}>
                      <Route path="users" element={<UsersPage />} />
                    </Route>
                    <Route path="profile" element={<ProfilePage />} />
                    <Route path="*" element={<Navigate to="/" replace />} />
                  </Route>
                </Route>
              </Routes>
            </Suspense>
          </BrowserRouter>
          <Toaster />
        </QueryClientProvider>
      </TooltipProvider>
    </ThemeProvider>
  );
}
