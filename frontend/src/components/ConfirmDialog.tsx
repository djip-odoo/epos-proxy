import Dialog, { DialogAction } from "./Dialog";

interface ConfirmDialogProps {
  title: string;
  message: string;
  openSignal?: number;
  onConfirm: () => unknown | Promise<unknown>;
  onClose?: () => void;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: "danger" | "primary";
}

export default function ConfirmDialog({
  title,
  message,
  openSignal,
  onConfirm,
  onClose,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  variant = "danger",
}: ConfirmDialogProps) {
  const actions: DialogAction[] = [
    {
      name: "cancel",
      label: cancelLabel,
      variant: "secondary",
      onClick: ({ close }) => {
        close();
      },
    },
    {
      name: "confirm",
      label: confirmLabel,
      variant: variant,
      onClick: async ({ close }) => {
        await onConfirm();
        close();
      },
    },
  ];

  return (
    <Dialog
      title={title}
      openSignal={openSignal}
      onClose={onClose}
      actions={actions}
    >
      <p className="text-sm text-gray-600">{message}</p>
    </Dialog>
  );
}
