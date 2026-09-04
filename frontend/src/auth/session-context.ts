import { createContext, useContext } from "react";

import type { Session } from "@/api/types";

export const SessionContext = createContext<Session | undefined>(undefined);

export function useSession(): Session {
  const session = useContext(SessionContext);
  if (!session) throw new Error("useSession 必须在 AuthBoundary 内使用");
  return session;
}
