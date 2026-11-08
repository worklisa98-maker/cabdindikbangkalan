type UpdateToastProps = {
  message?: string;
};

export function UpdateToast({ message }: UpdateToastProps) {
  if (!message) return null;
  return (
    <div
      className="fixed right-6 bottom-6 z-50 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 shadow-md"
      role="status"
      aria-live="polite"
    >
      {message}
    </div>
  );
}
