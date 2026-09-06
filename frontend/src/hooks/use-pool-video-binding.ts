import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { apiPost } from "@/api/client";
import { poolCreationWorkflow } from "@/lib/pool-creation";

export function usePoolVideoBinding<TResult>(source: string, open: boolean, poolID: (result: TResult) => number | string) {
  const [videoID, setVideoID] = useState("");
  const [offset, setOffset] = useState("0");
  const [workflow] = useState(() => poolCreationWorkflow<TResult>());
  const client = useQueryClient();
  useEffect(() => {
    if (!open) {
      setVideoID("");
      setOffset("0");
      workflow.reset();
    }
  }, [open, workflow]);

  return {
    videoID, setVideoID, offset, setOffset, open,
    async create(body: unknown, create: () => Promise<TResult>) {
      try {
        return await workflow.run(body, { videoID, offset }, create, (result, id, seconds) =>
          apiPost(`/api/admin/videos/${id}/${source}-bindings`, { poolId: poolID(result), offset: seconds }),
        );
      } finally {
        // Also refresh after partial success, so an already-created pool remains visible.
        await Promise.all(["videos", "video", "video-heatmap", `${source}-pools`, `${source}-pool-options`, "catalog-pool-options"].map((key) =>
          client.invalidateQueries({ queryKey: [key] }),
        ));
      }
    },
  };
}
