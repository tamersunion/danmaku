import { useId } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiGet } from "@/api/client";
import type { ApiResponse, ManagedVideo } from "@/api/types";
import { QueryError } from "@/components/query-error";
import { SearchableSelect } from "@/components/searchable-select";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

export function PoolVideoBindingFields({ binding, disabled }: {
  binding: { open: boolean; videoID: string; offset: string; setVideoID: (value: string) => void; setOffset: (value: string) => void };
  disabled: boolean;
}) {
  const id = useId();
  const videos = useQuery({
    queryKey: ["pool-creation-video-options"],
    enabled: binding.open,
    queryFn: async () => {
      const all: ManagedVideo[] = [];
      for (let page = 1; ; page++) {
        const { data } = await apiGet<ApiResponse<{ list: ManagedVideo[]; total: number }>>("/api/admin/videos", { page, size: 500, deleted: false });
        all.push(...data.list);
        if (!data.list.length || all.length >= data.total) return all;
      }
    },
  });
  return (
    <FieldGroup>
      <Field data-disabled={disabled || videos.isPending}>
        <FieldLabel htmlFor={`${id}-video`}>关联视频（可选）</FieldLabel>
        <SearchableSelect
          id={`${id}-video`}
          options={[{ value: "", label: "暂不关联" }, ...(videos.data ?? []).map((video) => ({ value: String(video.id), label: video.name ? `${video.name} (${video.vid})` : video.vid }))]}
          value={binding.videoID}
          onValueChange={binding.setVideoID}
          disabled={disabled || videos.isPending}
          placeholder={videos.isPending ? "正在加载视频" : "暂不关联"}
          searchPlaceholder="搜索视频名称或 ID"
        />
        <FieldDescription>选中后会在创建弹幕池时自动关联，留空仅创建弹幕池</FieldDescription>
        {videos.isError ? <QueryError error={videos.error} retry={() => void videos.refetch()} /> : null}
        {!videos.isPending && !videos.isError && videos.data?.length === 0 ? <FieldDescription>暂无可关联的视频，请先在视频管理中添加</FieldDescription> : null}
      </Field>
      {binding.videoID ? <Field data-disabled={disabled}>
        <FieldLabel htmlFor={`${id}-offset`}>关联偏移量（秒）</FieldLabel>
        <Input id={`${id}-offset`} type="number" step="any" required value={binding.offset} onChange={(event) => binding.setOffset(event.target.value)} disabled={disabled} />
        <FieldDescription>正数延后，负数提前，默认 0</FieldDescription>
      </Field> : null}
    </FieldGroup>
  );
}
