import { useQuery } from "@tanstack/react-query";
import { RefreshCwIcon, ServerCrashIcon } from "lucide-react";
import { Navigate, Outlet, useLocation } from "react-router-dom";

import { ApiClientError, apiGet, errorMessage } from "@/api/client";
import type {
  ApiResponse,
  AuthOptions,
  Session,
  SessionData,
} from "@/api/types";
import { SessionContext } from "@/auth/session-context";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export function AuthBoundary() {
  const location = useLocation();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: async (): Promise<Session> => {
      const [sessionResponse, optionsResponse] = await Promise.all([
        apiGet<ApiResponse<SessionData>>("/api/admin/session"),
        apiGet<ApiResponse<AuthOptions>>("/api/admin/auth/options"),
      ]);
      const current = sessionResponse.data;
      const options = optionsResponse.data;
      return {
        ...current,
        casEnabled: options.casEnabled,
        canManageDanmaku:
          current.role === "SuperAdmin" || current.role === "Admin",
        canManageUsers: current.role === "SuperAdmin",
        profileEditable:
          !options.casEnabled && current.provider !== "cas",
      };
    },
    staleTime: 60_000,
    retry: false,
  });

  if (session.isPending)
    return (
      <main
        className="min-h-svh bg-background"
        aria-busy="true"
        aria-label="正在读取登录状态"
      />
    );
  if (session.error instanceof ApiClientError && session.error.code === 401) {
    const redirect = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?redirect=${redirect}`} replace />;
  }
  if (session.isError) {
    return (
      <main className="grid min-h-svh place-items-center bg-muted/30 px-4">
        <Card className="w-full max-w-md">
          <CardHeader>
            <ServerCrashIcon className="text-primary" />
            <CardTitle>暂时无法进入控制台</CardTitle>
            <CardDescription>服务没有返回有效的登录状态。</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <Alert variant="destructive">
              <AlertTitle>读取失败</AlertTitle>
              <AlertDescription>{errorMessage(session.error)}</AlertDescription>
            </Alert>
            <Button variant="outline" onClick={() => void session.refetch()}>
              <RefreshCwIcon data-icon="inline-start" />
              重试
            </Button>
          </CardContent>
        </Card>
      </main>
    );
  }

  return (
    <SessionContext.Provider value={session.data}>
      <Outlet />
    </SessionContext.Provider>
  );
}
