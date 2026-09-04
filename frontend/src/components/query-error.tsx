import { RefreshCwIcon, ShieldAlertIcon } from "lucide-react";

import { errorMessage } from "@/api/client";
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert";
import { Button } from "@/components/ui/button";

export function QueryError({
  error,
  retry,
}: {
  error: unknown;
  retry: () => void;
}) {
  return (
    <Alert variant="destructive">
      <ShieldAlertIcon />
      <AlertTitle>数据加载失败</AlertTitle>
      <AlertDescription>{errorMessage(error)}</AlertDescription>
      <AlertAction>
        <Button size="sm" variant="outline" onClick={retry}>
          <RefreshCwIcon data-icon="inline-start" />
          重试
        </Button>
      </AlertAction>
    </Alert>
  );
}
