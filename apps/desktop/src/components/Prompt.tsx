import { useEffect, useRef, useState } from "react";

/**
 * A modal that asks for a string or a yes/no.
 *
 * Electron does not implement `window.prompt`, and a delete that silently did
 * nothing because `confirm` returned undefined would be a data-loss bug. One
 * modal covers both cases rather than two components or a dialog library.
 */
export type Ask = {
  title: string;
  message?: string;
  /** Present for a text question, absent for a confirmation. */
  initial?: string;
  placeholder?: string;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: (value: string) => void;
};

type Props = { ask: Ask; onCancel: () => void };

export default function Prompt({ ask, onCancel }: Props) {
  const [value, setValue] = useState(ask.initial ?? "");
  const input = useRef<HTMLInputElement>(null);
  const confirmButton = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    // Focus the field when there is one, the safe button when there is not —
    // so Enter never confirms a destructive action the user has not read.
    if (ask.initial !== undefined) input.current?.focus();
    else confirmButton.current?.focus();
  }, [ask]);

  const isText = ask.initial !== undefined;
  const disabled = isText && value.trim() === "";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      onMouseDown={onCancel}
      role="presentation"
    >
      <form
        className="w-96 max-w-[90vw] rounded border border-line bg-raised p-4 shadow-2xl"
        onMouseDown={(e) => e.stopPropagation()}
        onSubmit={(e) => {
          e.preventDefault();
          if (!disabled) ask.onConfirm(value.trim());
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") onCancel();
        }}
      >
        <h2 className="font-medium text-ink">{ask.title}</h2>
        {ask.message && <p className="mt-1 text-ink-dim">{ask.message}</p>}

        {isText && (
          <input
            ref={input}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={ask.placeholder}
            spellCheck={false}
            className="mt-3 w-full rounded border border-line bg-editor px-2 py-1 font-mono text-[11px] text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none"
          />
        )}

        <div className="mt-4 flex justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="rounded border border-line px-3 py-0.5 text-ink-dim hover:bg-hover hover:text-ink"
          >
            Cancel
          </button>
          <button
            ref={confirmButton}
            type="submit"
            disabled={disabled}
            className={`rounded px-3 py-0.5 font-medium text-accent-ink disabled:opacity-40 ${
              ask.danger ? "bg-danger" : "bg-accent"
            }`}
          >
            {ask.confirmLabel}
          </button>
        </div>
      </form>
    </div>
  );
}
