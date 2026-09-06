import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiGet } from "@/api/client";
import type { ApiResponse } from "@/api/types";
import { QueryError } from "@/components/query-error";
import { SearchableSelect } from "@/components/searchable-select";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Spinner } from "@/components/ui/spinner";

export function BilibiliLookup({ input, onSelect, disabled }: { input: string; onSelect: (reference: string) => void; disabled: boolean }) {
  const [resolvedInput, setResolvedInput] = useState("");
  const [selection, setSelection] = useState("");
  const result = useQuery({
    queryKey: ["bilibili-resolve", resolvedInput],
    queryFn: async () => (await apiGet<ApiResponse<{ key: string; reference: string; title: string; cid: number; p: number }[]>>("/api/admin/bilibili/resolve", {input: resolvedInput})).data,
    enabled: Boolean(resolvedInput), retry: false, staleTime: 60000, refetchOnWindowFocus: false,
  });
  const currentResult = input.trim() === resolvedInput || result.data?.some((item) => item.key === selection && item.reference === input.trim());
  return (
    <Field>
      <Button type="button" variant="outline" disabled={disabled || !input.trim() || result.isFetching} onClick={() => { setSelection(""); if (resolvedInput === input.trim()) void result.refetch(); else setResolvedInput(input.trim()); }}>
        {result.isFetching ? <Spinner data-icon="inline-start" /> : null}解析链接 / 标识
      </Button>
      {currentResult && result.isError ? <QueryError error={result.error} retry={() => void result.refetch()} /> : null}
      {currentResult && result.data ? <>
        <FieldLabel htmlFor="bilibili-resolved-part">选择分 P / 剧集</FieldLabel>
        <SearchableSelect id="bilibili-resolved-part" options={result.data.map((item) => ({value: item.key, label: `${item.title} · P${item.p} · CID ${item.cid}`}))} value={selection} disabled={disabled || result.isFetching} placeholder="选择要创建弹幕池的视频" onValueChange={(key) => { const item = result.data.find((item) => item.key === key); if (item) { setSelection(key); onSelect(item.reference); } }} />
      </> : null}
      <FieldDescription>支持 AV/BV、EP/SS、网页或手机客户端链接、b23.tv 短链接及分享文本；分 P 链接优先使用链接中的 p</FieldDescription>
    </Field>
  );
}
