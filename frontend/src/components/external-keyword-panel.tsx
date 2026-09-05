import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {PlusIcon,Trash2Icon} from "lucide-react";
import {apiGet,apiPost,apiDelete} from "@/api/client";
import type {ApiResponse,ExternalPool} from "@/api/types";
import {DataTable,type DataColumn} from "@/components/data-table";
import {ConfirmAction} from "@/components/confirm-action";
import {QueryError} from "@/components/query-error";
import {LoadingTable} from "@/components/loading-table";
import {SearchableSelect} from "@/components/searchable-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card,CardHeader,CardTitle,CardDescription,CardContent} from "@/components/ui/card";
import {Field,FieldGroup,FieldLabel} from "@/components/ui/field";
import {Input} from "@/components/ui/input";
import {Spinner} from "@/components/ui/spinner";
import {useApiMutation} from "@/hooks/use-api-mutation";
import {formatDateTime} from "@/lib/format";
type ExternalKeyword={id:number;poolId:string|null;poolName:string;keyword:string;createTime:string};

export function ExternalKeywordPanel({ pools }: { pools: ExternalPool[] }) {
  const [scope, setScope] = useState("global");
  const [keyword, setKeyword] = useState("");
  const keywords = useQuery({
    queryKey: ["external-keywords"],
    queryFn: async () => (await apiGet<ApiResponse<ExternalKeyword[]>>("/api/admin/external/keywords")).data,
  });
  const create = useApiMutation<
    { poolId: string | null; keyword: string },
    ApiResponse<ExternalKeyword>
  >({
    mutationFn: (body) => apiPost("/api/admin/external/keywords", body),
    successMessage: "过滤关键词已添加",
    invalidate: [["external-keywords"], ["external-pools"], ["external-pool-danmaku"], ["video-heatmap"]],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: (id) => apiDelete(`/api/admin/external/keywords/${id}`),
    successMessage: "过滤关键词已删除",
    invalidate: [["external-keywords"], ["external-pools"], ["external-pool-danmaku"], ["video-heatmap"]],
  });
  const columns: DataColumn<ExternalKeyword>[] = [
    { key: "keyword", label: "关键词", render: (item) => <span className="font-medium">{item.keyword}</span> },
    { key: "scope", label: "作用范围", render: (item) => <Badge variant={item.poolId ? "secondary" : "default"}>{item.poolId ? item.poolName : "全局"}</Badge> },
    { key: "created", label: "创建时间", className: "whitespace-nowrap", render: (item) => formatDateTime(item.createTime) },
    {
      key: "actions",
      label: "操作",
      className: "w-16 text-right",
      render: (item) => (
        <ConfirmAction
          trigger={<Button type="button" size="icon-sm" variant="destructive" aria-label="删除过滤关键词"><Trash2Icon /></Button>}
          title="删除这个过滤关键词？"
          description="删除后，匹配的弹幕将不再因这条规则被自动屏蔽。"
          destructive
          pending={remove.isPending}
          onConfirm={() => remove.mutate(item.id)}
        />
      ),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>过滤关键词</CardTitle>
        <CardDescription>全局规则作用于所有外部导入弹幕池；池级规则只作用于指定弹幕池。弹幕数据仍会完整保留。</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            create.mutate(
              { poolId: scope === "global" ? null : scope, keyword: keyword.trim() },
              { onSuccess: () => setKeyword("") },
            );
          }}
        >
          <FieldGroup className="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <Field>
              <FieldLabel htmlFor="external-keyword-scope">作用范围</FieldLabel>
              <SearchableSelect
                id="external-keyword-scope"
                options={[
                  { value: "global", label: "全部弹幕池" },
                  ...pools.map((pool) => ({ value: String(pool.id), label: pool.name })),
                ]}
                value={scope}
                searchPlaceholder="搜索弹幕池"
                onValueChange={setScope}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="external-keyword-value">关键词</FieldLabel>
              <div className="flex gap-2">
                <Input id="external-keyword-value" value={keyword} maxLength={200} required onChange={(event) => setKeyword(event.target.value)} />
                <Button type="submit" disabled={create.isPending || !keyword.trim()}>
                  {create.isPending ? <Spinner data-icon="inline-start" /> : <PlusIcon data-icon="inline-start" />}
                  添加
                </Button>
              </div>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
      <CardContent className="p-0">
        {keywords.isError ? (
          <div className="p-6"><QueryError error={keywords.error} retry={() => void keywords.refetch()} /></div>
        ) : keywords.isPending ? (
          <LoadingTable />
        ) : (
          <DataTable rows={keywords.data ?? []} columns={columns} rowKey={(item) => String(item.id)} emptyTitle="暂无过滤关键词" emptyDescription="添加全局或弹幕池级关键词规则。" />
        )}
      </CardContent>
    </Card>
  );
}
