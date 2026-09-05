import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { PlusIcon, Trash2Icon } from "lucide-react";
import { apiGet, apiPost, apiDelete } from "@/api/client";
import type { ApiResponse } from "@/api/types";
import { ConfirmAction } from "@/components/confirm-action";
import { DataTable, type DataColumn } from "@/components/data-table";
import { LoadingTable } from "@/components/loading-table";
import { QueryError } from "@/components/query-error";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Spinner } from "@/components/ui/spinner";
import { useApiMutation } from "@/hooks/use-api-mutation";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";

type RuleKind = "keywords" | "authors" | "ips";
type Rule = { id: number; kind: RuleKind; value: string; replacement: string; createTime: string };
const settings = {
  keywords: { title: "关键词过滤", label: "关键词", placeholder: "输入需要过滤的内容", description: "不区分大小写的包含匹配。新提交命中后保存为已删除，接口仍返回成功；不扫描历史弹幕，删除规则也不会自动恢复已删除弹幕。" },
  authors: { title: "用户名映射", label: "原用户名", placeholder: "a", description: "按对外下发的用户名精确匹配，区分大小写，只映射一次后保存。可选择替换系统弹幕库的全部历史记录（包括已删除记录），不修改平台用户或第三方弹幕。" },
  ips: { title: "IP 黑名单", label: "IP 地址或 CIDR 网段", placeholder: "192.0.2.1、192.0.2.0/24 或 2001:db8::/32", description: "支持 IPv4、IPv6 和 CIDR 网段。命中后拒绝写入，只返回通用失败信息；不影响历史弹幕读取。" },
};

export function NativeRulePanel({ kind }: { kind: RuleKind }) {
  const config = settings[kind];
  const [value, setValue] = useState("");
  const [replacement, setReplacement] = useState("");
  const [scanExisting, setScanExisting] = useState(false);
  const [replaced, setReplaced] = useState<number | null>(null);
  const rules = useQuery({ queryKey: ["native-rules", kind], queryFn: async () => (await apiGet<ApiResponse<Rule[]>>(`/api/admin/danmaku-rules/${kind}`)).data });
  const create = useApiMutation<{ value: string; replacement: string; scanExisting: boolean }, ApiResponse<{ rule: Rule; replaced: number }>>({
    mutationFn: body => apiPost(`/api/admin/danmaku-rules/${kind}`, body),
    successMessage: "规则已创建",
    invalidate: [["native-rules", kind], ["danmaku"], ["danmaku-detail"], ["video-heatmap"]],
  });
  const remove = useApiMutation<number, ApiResponse<null>>({
    mutationFn: id => apiDelete(`/api/admin/danmaku-rules/${kind}/${id}`),
    successMessage: "规则已删除",
    invalidate: [["native-rules", kind]],
  });
  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ value, replacement: kind === "authors" ? replacement : "", scanExisting: kind === "authors" && scanExisting }, {
      onSuccess: response => { setValue(""); setReplacement(""); setReplaced(scanExisting ? response.data.replaced : null); setScanExisting(false); },
    });
  }
  const columns: DataColumn<Rule>[] = [
    { key: "value", label: config.label, render: item => <span className="break-all">{item.value}</span> },
    ...(kind === "authors" ? [{ key: "replacement", label: "映射为", render: (item: Rule) => <span className="break-all">{item.replacement}</span> }] : []),
    { key: "created", label: "创建时间", className: "whitespace-nowrap", render: item => formatDateTime(item.createTime) },
    { key: "actions", label: "操作", className: "w-16 text-right", render: item => <ConfirmAction
      trigger={<Button type="button" size="icon-sm" variant="destructive" aria-label="删除规则"><Trash2Icon /></Button>}
      title="删除这条规则？" description="删除只影响后续提交，不会还原已保存的用户名或软删除状态。"
      destructive pending={remove.isPending} onConfirm={() => remove.mutate(item.id)} /> },
  ];
  return <Card>
    <CardHeader><CardTitle>{config.title}</CardTitle><CardDescription>{config.description}</CardDescription></CardHeader>
    <CardContent>
      <form onSubmit={submit}>
        <FieldGroup>
          <FieldGroup className={cn(kind === "authors" && "grid gap-4 sm:grid-cols-2")}>
            <Field data-disabled={create.isPending}><FieldLabel htmlFor={`${kind}-value`}>{config.label}</FieldLabel><Input id={`${kind}-value`} value={value} onChange={event => setValue(event.target.value)} placeholder={config.placeholder} maxLength={200} required disabled={create.isPending} /></Field>
            {kind === "authors" ? <Field data-disabled={create.isPending}><FieldLabel htmlFor="authors-replacement">映射为</FieldLabel><Input id="authors-replacement" value={replacement} onChange={event => setReplacement(event.target.value)} placeholder="b" maxLength={200} required disabled={create.isPending} /></Field> : null}
          </FieldGroup>
          {kind === "authors" ? <Field orientation="horizontal" data-disabled={create.isPending}>
            <FieldContent><FieldLabel htmlFor="authors-scan">同时替换全库已有弹幕</FieldLabel><FieldDescription>默认关闭。开启后立即替换所有匹配记录，删除映射规则不会撤销替换。</FieldDescription></FieldContent>
            <Switch id="authors-scan" checked={scanExisting} onCheckedChange={setScanExisting} disabled={create.isPending} />
          </Field> : null}
          <Button type="submit" className="self-end" disabled={create.isPending || !value.trim() || (kind === "authors" && !replacement.trim())}>{create.isPending ? <Spinner data-icon="inline-start" /> : <PlusIcon data-icon="inline-start" />}添加规则</Button>
          {replaced !== null ? <FieldDescription role="status">已替换 {replaced} 条历史弹幕的用户名。</FieldDescription> : null}
        </FieldGroup>
      </form>
    </CardContent>
    <CardContent className="p-0">{rules.isError ? <div className="p-6"><QueryError error={rules.error} retry={() => void rules.refetch()} /></div> : rules.isPending ? <LoadingTable /> : <DataTable rows={rules.data ?? []} columns={columns} rowKey={item => String(item.id)} emptyTitle="暂无规则" emptyDescription="添加规则后对新的系统弹幕提交生效。" />}</CardContent>
  </Card>;
}
