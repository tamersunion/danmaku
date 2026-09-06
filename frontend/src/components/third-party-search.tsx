import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { SearchIcon } from "lucide-react";

import { apiGet } from "@/api/client";
import type { ApiResponse } from "@/api/types";
import { QueryError } from "@/components/query-error";
import { SearchableSelect } from "@/components/searchable-select";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";

type Anime = { animeId: string; title: string; typeDescription: string; startDate: string };
type Episode = { episodeId: string; title: string; number: string };

export function ThirdPartySearch({ source, episodeId, onSelect, disabled = false }: {
  source: "animeko" | "iqiyi" | "bilibili" | "bahamut" | "tencent" | "youku";
  episodeId: string;
  onSelect: (episodeId: string) => void;
  disabled?: boolean;
}) {
  const [kind, setKind] = useState("video");
  const [draft, setDraft] = useState("");
  const [keyword, setKeyword] = useState("");
  const [animeId, setAnimeId] = useState("");
  const search = useQuery({
    queryKey: [source, "search", keyword, kind],
    queryFn: async () => (await apiGet<ApiResponse<Anime[]>>(`/api/admin/${source}/search`, { keyword, ...(source === "bilibili" ? { type: kind } : {}) })).data,
    enabled: Boolean(keyword),
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: false,
  });
  const episodes = useQuery({
    queryKey: [source, "episodes", animeId],
    queryFn: async () => {
      if (source === "bilibili") {
        const data = (await apiGet<ApiResponse<{reference: string; title: string; p: number}[]>>("/api/admin/bilibili/resolve", {input: animeId})).data;
        return data.map((item) => ({episodeId: item.reference, title: item.title, number: String(item.p)}));
      }
      return (await apiGet<ApiResponse<Episode[]>>(`/api/admin/${source}/anime/${animeId}/episodes`)).data;
    },
    enabled: Boolean(animeId),
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: false,
  });
  function submitSearch() {
    if (!draft.trim() || disabled || search.isFetching) return;
    setAnimeId("");
    onSelect("");
    if (keyword === draft.trim()) void search.refetch();
    else setKeyword(draft.trim());
  }

  return (
    <FieldGroup>
      {source === "bilibili" ? <Field><FieldLabel htmlFor="bilibili-search-type">搜索类型</FieldLabel><SearchableSelect id="bilibili-search-type" options={[{value: "video", label: "普通视频"}, {value: "media_bangumi", label: "番剧"}, {value: "media_ft", label: "影视"}]} value={kind} disabled={disabled} onValueChange={(value) => { setKind(value); setKeyword(""); setAnimeId(""); onSelect(""); }} /></Field> : null}
      <Field data-disabled={disabled}>
        <FieldLabel htmlFor={`${source}-search-keyword`}>关键词搜索</FieldLabel>
        <div className="flex gap-2">
          <Input
            id={`${source}-search-keyword`}
            value={draft}
            disabled={disabled}
            maxLength={200}
            placeholder="输入作品名称"
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.nativeEvent.isComposing) {
                event.preventDefault();
                submitSearch();
              }
            }}
          />
          <Button type="button" variant="outline" disabled={disabled || !draft.trim() || search.isFetching} onClick={submitSearch}>
            {search.isFetching ? <Spinner data-icon="inline-start" /> : <SearchIcon data-icon="inline-start" />}
            搜索
          </Button>
        </div>
        <FieldDescription>搜索作品并选择剧集后自动填入 ID，也可以在下方直接输入</FieldDescription>
      </Field>
      {search.isError ? <QueryError error={search.error} retry={() => void search.refetch()} /> : null}
      {keyword && search.isSuccess && !search.isFetching ? (
        <Field data-disabled={disabled || !search.data.length}>
          <FieldLabel htmlFor={`${source}-search-anime`}>作品</FieldLabel>
          <SearchableSelect
            id={`${source}-search-anime`}
            options={search.data.map((anime) => ({
              value: anime.animeId,
              label: [anime.title, anime.typeDescription, /^\d{4}/.test(anime.startDate) ? anime.startDate.slice(0, 4) : "", `作品 ID ${anime.animeId}`].filter(Boolean).join(" · "),
            }))}
            value={animeId}
            disabled={disabled || !search.data.length}
            placeholder={search.data.length ? "选择作品" : "没有找到作品，请尝试其他名称"}
            searchPlaceholder="筛选作品"
            onValueChange={(value) => { setAnimeId(value); onSelect(""); }}
          />
          <FieldDescription>请选择对应的作品和季度，搜索不会自动创建弹幕池</FieldDescription>
        </Field>
      ) : null}
      {episodes.isError && animeId ? <QueryError error={episodes.error} retry={() => void episodes.refetch()} /> : null}
      {animeId ? (
        <Field data-disabled={disabled || episodes.isFetching || !episodes.data?.length}>
          <FieldLabel htmlFor={`${source}-search-episode`}>剧集</FieldLabel>
          <SearchableSelect
            id={`${source}-search-episode`}
            options={(episodes.data ?? []).map((episode) => ({
              value: episode.episodeId,
              label: [episode.number ? `第 ${episode.number} 集` : "", episode.title, `ID ${episode.episodeId}`].filter(Boolean).join(" · "),
            }))}
            value={(episodes.data ?? []).some((episode) => episode.episodeId === episodeId) ? episodeId : ""}
            disabled={disabled || episodes.isFetching || !episodes.data?.length}
            placeholder={episodes.isFetching ? "正在加载剧集" : episodes.data?.length ? "选择剧集" : "暂无剧集"}
            searchPlaceholder="搜索集数或剧集名称"
            onValueChange={onSelect}
          />
        </Field>
      ) : null}
    </FieldGroup>
  );
}
