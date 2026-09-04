import {
  useMutation,
  useQueryClient,
  type QueryKey,
} from "@tanstack/react-query";

import { errorMessage } from "@/api/client";
import { toast } from "@/components/ui/toast";

export function useApiMutation<TVariables, TResult = unknown>({
  mutationFn,
  successMessage,
  invalidate = [],
}: {
  mutationFn: (variables: TVariables) => Promise<TResult>;
  successMessage: string;
  invalidate?: QueryKey[];
}) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (variables: TVariables) =>
      toast.promise(
        Promise.resolve().then(() => mutationFn(variables)),
        {
          loading: {
            title: "正在处理操作",
            description: "请求已提交，请稍候。",
          },
          success: { title: successMessage },
          error: (error) => ({
            title: "操作失败",
            description: errorMessage(error),
          }),
        },
      ),
    onSuccess: async () => {
      await Promise.all(
        invalidate.map((queryKey) => client.invalidateQueries({ queryKey })),
      );
    },
  });
}
