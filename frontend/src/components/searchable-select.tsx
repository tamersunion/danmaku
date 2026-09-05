import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import { cn } from "@/lib/utils";

export type SearchableSelectOption = {
  value: string;
  label: string;
};

export function SearchableSelect({
  id,
  options,
  value,
  onValueChange,
  placeholder = "请选择",
  searchPlaceholder = "搜索选项",
  emptyText = "没有匹配的选项",
  disabled = false,
  className,
}: {
  id?: string;
  options: SearchableSelectOption[];
  value: string;
  onValueChange: (value: string) => void;
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  disabled?: boolean;
  className?: string;
}) {
  const selected =
    options.find((option) => option.value === value) ??
    (value ? { value, label: value } : null);
  const items =
    selected && !options.some((option) => option.value === selected.value)
      ? [selected, ...options]
      : options;

  return (
    <Combobox
      items={items}
      value={selected}
      disabled={disabled}
      itemToStringValue={(option) => option.label}
      isItemEqualToValue={(option, current) => option.value === current.value}
      onValueChange={(option) => {
        if (option) onValueChange(option.value);
      }}
    >
      <ComboboxInput
        id={id}
        className={cn("w-full", className)}
        placeholder={selected ? searchPlaceholder : placeholder}
        disabled={disabled}
      />
      <ComboboxContent>
        <ComboboxEmpty>{emptyText}</ComboboxEmpty>
        <ComboboxList>
          {(option) => (
            <ComboboxItem key={option.value} value={option}>
              {option.label}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}
