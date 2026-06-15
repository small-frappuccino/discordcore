import React, { useEffect } from "react";
import { Button } from "..";

interface ConfirmationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: React.ReactNode;
  confirmText?: string;
  cancelText?: string;
  isConfirming?: boolean;
}

export function ConfirmationModal({
  isOpen,
  onClose,
  onConfirm,
  title,
  description,
  confirmText = "Confirm",
  cancelText = "Cancel",
  isConfirming = false,
}: ConfirmationModalProps) {
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div 
        className="absolute inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
        onClick={() => !isConfirming && onClose()}
      />
      
      {/* Modal Box */}
      <div className="relative bg-[var(--bg-surface)] w-full max-w-md rounded-xl shadow-2xl overflow-hidden flex flex-col mx-4 transform transition-all">
        <div className="p-6">
          <h2 className="text-xl font-bold text-[var(--text-primary)] mb-3">{title}</h2>
          <div className="text-[var(--text-secondary)] text-sm leading-relaxed">
            {description}
          </div>
        </div>
        
        <div className="bg-[var(--bg-surface-hover)] p-4 flex items-center justify-end gap-3 border-t border-[var(--border-subtle)]">
          <Button 
            variant="ghost" 
            onClick={onClose} 
            disabled={isConfirming}
          >
            {cancelText}
          </Button>
          <Button 
            variant="danger" 
            onClick={onConfirm} 
            isLoading={isConfirming}
            disabled={isConfirming}
          >
            {confirmText}
          </Button>
        </div>
      </div>
    </div>
  );
}
