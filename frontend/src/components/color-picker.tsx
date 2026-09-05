import { useState } from "react";
import { ChevronDownIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "@/components/ui/popover";
import { Slider } from "@/components/ui/slider";
import { normalizeHex, replaceColorChannel } from "@/lib/picker-values";

const presets = ["#ffffff", "#000000", "#ff4d4f", "#ff7a45", "#ffc53d", "#73d13d", "#36cfc9", "#4096ff", "#9254de", "#f759ab"];

export function ColorPicker({ id, value, onChange, disabled = false }: { id: string; value: string; onChange: (value: string) => void; disabled?: boolean }) {
  const color = normalizeHex(value) ?? "#ffffff";
  const [open, setOpen] = useState(false);
  const [hex, setHex] = useState(color);
  const choose = (next: string) => { onChange(next); setHex(next); };
  return <Popover open={open && !disabled} onOpenChange={next => {setOpen(next); if (next) setHex(color);}}>
    <PopoverTrigger render={<Button id={id} type="button" variant="outline" disabled={disabled} className="w-full justify-start" />}>
      <span className="size-4 shrink-0 rounded-sm border" style={{backgroundColor: color}} aria-hidden="true" />
      <span className="font-mono">{color.toUpperCase()}</span><ChevronDownIcon data-icon="inline-end" className="ml-auto" />
    </PopoverTrigger>
    <PopoverContent align="start" className="w-72 max-w-[calc(100vw-2rem)] gap-4 p-4">
      <PopoverTitle>选择弹幕颜色</PopoverTitle>
      <div className="h-12 rounded-lg border" style={{backgroundColor:color}} aria-label={`当前颜色 ${color}`} />
      <div className="grid grid-cols-5 gap-2" role="group" aria-label="常用颜色">
        {presets.map(preset => <Button key={preset} type="button" variant="outline" size="icon-sm" className="w-full" aria-label={`选择 ${preset}`} aria-pressed={color === preset} title={preset} onClick={() => choose(preset)}><span className="size-4 rounded-sm border" style={{backgroundColor:preset}} /></Button>)}
      </div>
      <FieldGroup className="gap-4">
        {["R", "G", "B"].map((label, index) => <Field key={label}>
          <FieldLabel htmlFor={`${id}-${label}`}>{label}<span className="ml-auto font-mono">{Number.parseInt(color.slice(1+index*2, 3+index*2), 16)}</span></FieldLabel>
          <Slider id={`${id}-${label}`} thumbLabel={`${label} 通道`} min={0} max={255} step={1} value={[Number.parseInt(color.slice(1+index*2, 3+index*2), 16)]} onValueChange={next => choose(replaceColorChannel(color, index, Array.isArray(next) ? next[0] : next))} />
        </Field>)}
        <Field data-invalid={!normalizeHex(hex)}>
          <FieldLabel htmlFor={`${id}-hex`}>HEX 颜色值</FieldLabel>
          <Input id={`${id}-hex`} value={hex} maxLength={7} aria-invalid={!normalizeHex(hex)} spellCheck={false} onChange={event => { setHex(event.target.value); if (/^#?[\da-f]{6}$/i.test(event.target.value)) onChange(normalizeHex(event.target.value)!); }} onBlur={() => { const next = normalizeHex(hex); choose(next ?? color); }} onKeyDown={event => { if (event.key === "Enter") { event.preventDefault(); const next = normalizeHex(hex); if (next) choose(next); } }} />
          {!normalizeHex(hex) ? <FieldDescription>请输入 3 位或 6 位十六进制颜色值</FieldDescription> : null}
        </Field>
      </FieldGroup>
      <Button type="button" className="self-end" onClick={() => setOpen(false)}>完成</Button>
    </PopoverContent>
  </Popover>;
}
